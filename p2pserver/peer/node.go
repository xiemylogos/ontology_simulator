package peer

import (
	"errors"
	"fmt"
	"sync"

	glog "log"

	"github.com/ontio/ontology/common/log"
)

type Noder interface {
	GetID() string
	GetConfig() *NodeConfig
	GetNeighbours() []string
	GetLogger() *log.Logger
	Broadcast(msgType uint64, msgData []byte) error
}

type PeerConnection interface {
	GetLocalID() string
	GetRemoteID() string
	Send(msgType uint64, msgData []byte) error
	Receive() (uint64, []byte, error)
}

type Service interface {
	Start(node Noder) error
	Stop() error
	Info() string
	Run(peer PeerConnection) error
}

// Services is a collection of services which can be run in a simulation
type Services map[string]ServiceFunc

// ServiceFunc returns a node.Service which can be used to boot a devp2p node
type ServiceFunc func(node Noder) (Service, error)

// serviceFuncs is a map of registered services which are used to boot devp2p
// nodes
var serviceFuncs = make(Services)

// RegisterServices registers the given Services which can then be used to
// start devp2p nodes using either the Exec or Docker adapters.
//
// It should be called in an init function so that it has the opportunity to
// execute the services before main() is called.
func RegisterServices(services Services) {
	for name, f := range services {
		if _, exists := serviceFuncs[name]; exists {
			panic(fmt.Sprintf("node service already exists: %q", name))
		}
		serviceFuncs[name] = f
	}
}

type Node struct {
	ID           string
	network      *Network
	Config       *NodeConfig `json:"config"`
	Up           bool        `json:"up"`
	registerOnce sync.Once

	logger *log.Logger

	services []Service
	conns    map[*Conn]*PeerConn

	quitC chan struct{}
}

type PeerConn struct {
	local    *Node
	remote   *Node
	conn     *Conn
	ReceiveC chan *PeerEvent
}

func NewNode(id string, network *Network, conf *NodeConfig) (*Node, error) {

	prefix := id + ": "
	logger := log.New(log.Stdout, prefix, glog.Ldate|glog.Lmicroseconds, 1, nil)

	return &Node{
		ID:      id,
		network: network,
		Config:  conf,
		logger:  logger,
		conns:   make(map[*Conn]*PeerConn),
		quitC:   make(chan struct{}),
	}, nil
}

func (self *Node) GetID() string {
	return self.ID
}

func (self *Node) GetConfig() *NodeConfig {
	return self.Config
}

func (self *Node) GetLogger() *log.Logger {
	return self.logger
}

func (self *Node) Start() error {

	if self.network == nil {
		return fmt.Errorf("node has not been initialized")
	}

	// ensure we only register the services once in the case of the node
	// being stopped and then started again
	self.registerOnce.Do(func() {
		for _, name := range self.Config.Services {
			f := serviceFuncs[name]
			s, err := f(self)
			if err != nil {
				return
			}
			self.services = append(self.services, s)
		}
	})

	for _, s := range self.services {
		s.Start(self)
	}

	return nil
}

func (self *Node) Stop() error {

	for _, s := range self.services {
		s.Stop()
	}

	self.quitC <- struct{}{}

	return nil
}

func (self *Node) Broadcast(msgType uint64, payload []byte) error {
	return self.network.didBroadcast(self.ID, msgType, payload)
}

func (self *Node) receive(conn *Conn, msgType uint64, msgData []byte) error {
	if peerconn := self.conns[conn]; peerconn != nil {
		log.Debugf("node %s, received msg from.", self.ID, peerconn.remote.ID)
		peerconn.ReceiveC <- &PeerEvent{
			MsgType: msgType,
			MsgData: msgData,
		}
		return nil
	}
	return fmt.Errorf("peer connection not available")
}

func (self *Node) GetNeighbours() []string {
	return self.network.GetNeighbours(self.ID)
}

func (self *Node) PeerConnected(conn *Conn) error {
	if c := self.conns[conn]; c != nil {
		log.Debugf("connection already added: %s, remote: %s", self.ID, conn.Other)
		return fmt.Errorf("connection already added: %s, remote: %s", self.ID, conn.Other)
	}

	remote := conn.other
	if self.ID == remote.ID {
		remote = conn.one
	}

	log.Debugf("Node %s connected with %s", self.ID, remote.ID)

	peerConn := &PeerConn{
		local:    self,
		remote:   remote,
		conn:     conn,
		ReceiveC: make(chan *PeerEvent, 1024),
	}
	self.conns[conn] = peerConn

	for _, s := range self.services {
		go func() {
			fmt.Println("xiexie--connect------:", self.ID, remote.ID, len(self.services), s.Info())
			if err := s.Run(peerConn); err != nil {
				log.Errorf("run service failed: %s", err)
			}
		}()
	}

	return nil
}

func (self *Node) PeerDisconnected(conn *Conn) error {
	if _, ok := self.conns[conn]; !ok {
		log.Debugf("disconnected failed: %s, remote: %s", self.ID, conn.Other)
		return fmt.Errorf("disconnected failed: %s, remote: %s", self.ID, conn.Other)
	}
	delete(self.conns, conn)
	return nil
}

func (self *PeerConn) GetLocalID() string {
	return self.local.ID
}

func (self *PeerConn) GetRemoteID() string {
	return self.remote.ID
}

func (self *PeerConn) Send(msgtype uint64, msgdata []byte) error {
	return self.local.network.DidSend(self.local.GetID(), self.remote.GetID(), msgtype, msgdata)
}

func (self *PeerConn) Receive() (uint64, []byte, error) {
	select {
	case msg := <-self.ReceiveC:
		log.Debugf("msg received from %s to %s", self.remote.ID, self.local.ID)
		if msg.Error != "" {
			return msg.MsgType, msg.MsgData, errors.New(msg.Error)
		}
		return msg.MsgType, msg.MsgData, nil
	}
	return 0, nil, nil
}
