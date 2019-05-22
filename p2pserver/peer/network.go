package peer

import (
	"fmt"
	"sync"

	"github.com/ontio/ontology/common/log"
	"github.com/ontio/ontology/events"
)

// NetworkConfig defines configuration options for starting a Network
type NetworkConfig struct {
	ID             string `json:"id"`
	DefaultService string `json:"default_service,omitempty"`
}

// Network models a p2p simulation network which consists of a collection of
// simulated nodes and the connections which exist between them.
//
// The Network has a single NodeAdapter which is responsible for actually
// starting nodes and connecting them together.
//
// The Network emits events when nodes are started and stopped, when they are
// connected and disconnected, and also when messages are sent between nodes.
type Network struct {
	NetworkConfig

	Nodes   []*Node `json:"nodes"`
	nodeMap map[string]chan *PeerEvent

	Conns   []*Conn `json:"conns"`
	connMap map[string]int

	events    *events.Event
	eventchan chan interface{}
	lock      sync.RWMutex
	quitc     chan struct{}
}

// NewNetwork returns a Network which uses the given NodeAdapter and NetworkConfig
func NewNetwork(conf *NetworkConfig) *Network {
	return &Network{
		NetworkConfig: *conf,
		nodeMap:       make(map[string]chan *PeerEvent),
		connMap:       make(map[string]int),
		events:        events.NewEvent(),
		quitc:         make(chan struct{}),
	}
}

// StartAll starts all nodes in the network
func (self *Network) StartAll() error {
	for _, node := range self.Nodes {
		if node.Up {
			continue
		}
		if err := self.Start(node.ID); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all nodes in the network
func (self *Network) StopAll() error {
	for _, node := range self.Nodes {
		if !node.Up {
			continue
		}
		if err := self.Stop(node.ID); err != nil {
			return err
		}
	}
	return nil
}

func (self *Network) Start(id string) error {
	node := self.GetNode(id)
	if node == nil {
		return fmt.Errorf("node %v does not exist", id)
	}
	if node.Up {
		return fmt.Errorf("node %v already up", id)
	}

	log.Debug(fmt.Sprintf("starting node %v: %v ", id, node.Up))
	if err := node.Start(); err != nil {
		log.Warn(fmt.Sprintf("start up failed: %v", err))
		return err
	}

	node.Up = true
	log.Info(fmt.Sprintf("started node %v: %v", id, node.Up))

	peerEventChan := make(chan *PeerEvent)
	self.nodeMap[id] = peerEventChan

	go self.watchPeerEvents(id, peerEventChan)
	return nil
}

// Stop stops the node with the given ID
func (self *Network) Stop(id string) error {
	node := self.GetNode(id)
	if node == nil {
		return fmt.Errorf("node %v does not exist", id)
	}
	if !node.Up {
		return fmt.Errorf("node %v already down", id)
	}
	if err := node.Stop(); err != nil {
		return err
	}
	node.Up = false
	log.Info(fmt.Sprintf("stop node %v: %v", id, node.Up))

	return node.Stop()
}

// Connect connects two nodes together by calling the "admin_addPeer" RPC
// method on the "one" node so that it connects to the "other" node
func (self *Network) Connect(oneID, otherID string) error {
	log.Debug(fmt.Sprintf("connecting %s to %s", oneID, otherID))
	conn, err := self.GetOrCreateConn(oneID, otherID)
	if err != nil {
		return err
	}
	if conn.Up {
		return fmt.Errorf("%v and %v already connected", oneID, otherID)
	}
	if err := conn.nodesUp(); err != nil {
		return err
	}

	self.nodeMap[conn.One] <- &PeerEvent{
		Type: PeerEventTypeAdd,
		Peer: conn.Other,
	}
	return nil
}

// Disconnect disconnects two nodes by calling the "admin_removePeer" RPC
// method on the "one" node so that it disconnects from the "other" node
func (self *Network) Disconnect(oneID, otherID string) error {
	conn := self.GetConn(oneID, otherID)
	if conn == nil {
		return fmt.Errorf("connection between %v and %v does not exist", oneID, otherID)
	}
	if !conn.Up {
		return fmt.Errorf("%v and %v already disconnected", oneID, otherID)
	}

	self.nodeMap[conn.One] <- &PeerEvent{
		Type: PeerEventTypeDrop,
		Peer: conn.other.ID,
	}

	return nil
}

// DidConnect tracks the fact that the "one" node connected to the "other" node
func (self *Network) DidConnect(one, other string) error {

	log.Debugf("network connecting %s and %s", one, other)

	conn, err := self.GetOrCreateConn(one, other)
	if err != nil {
		return fmt.Errorf("connection between %v and %v does not exist", one, other)
	}
	if conn.Up {
		return fmt.Errorf("%v and %v already connected", one, other)
	}
	conn.TurnUp()

	return nil
}

// DidDisconnect tracks the fact that the "one" node disconnected from the
// "other" node
func (self *Network) DidDisconnect(one, other string) error {
	conn, err := self.GetOrCreateConn(one, other)
	if err != nil {
		return fmt.Errorf("connection between %v and %v does not exist", one, other)
	}
	if !conn.Up {
		return fmt.Errorf("%v and %v already disconnected", one, other)
	}
	conn.TurnDown()
	return nil
}

// DidSend tracks the fact that "sender" sent a message to "receiver"
func (self *Network) DidSend(sender, receiver string, msgtype uint64, payload []byte) error {

	n := self.getNode(receiver)
	if n == nil || !n.Up {
		return fmt.Errorf("receiver not available or is down")
	}

	if conn := self.getConn(sender, receiver); conn != nil && conn.Up {
		if err := n.receive(conn, msgtype, payload); err != nil {
			log.Errorf("network failed to send msg: %s", err)
		}
	} else {
		return fmt.Errorf("connection not available or is down")
	}

	return nil
}

func (self *Network) didBroadcast(sender string, msgtype uint64, payload []byte) error {
	sendCount := 0
	for _, n := range self.Nodes {
		if conn := self.getConn(sender, n.ID); conn != nil && conn.Up {
			if err := n.receive(conn, msgtype, payload); err != nil {
				log.Errorf("network failed to broadcast msg: %s", err)
			}
			sendCount++
		}
	}

	if sendCount == 0 {
		log.Errorf("Failed to broadcast on any connection!!")
	}

	return nil
}

// GetNode gets the node with the given ID, returning nil if the node does not
// exist
func (self *Network) GetNode(id string) *Node {
	self.lock.Lock()
	defer self.lock.Unlock()
	return self.getNode(id)
}

func (self *Network) getNode(id string) *Node {
	for _, node := range self.Nodes {
		if id == node.ID {
			return node
		}
	}
	return nil
}

// GetConn returns the connection which exists between "one" and "other"
// regardless of which node initiated the connection
func (self *Network) GetConn(oneID, otherID string) *Conn {
	self.lock.Lock()
	defer self.lock.Unlock()
	return self.getConn(oneID, otherID)
}

// GetOrCreateConn is like GetConn but creates the connection if it doesn't
// already exist
func (self *Network) GetOrCreateConn(oneID, otherID string) (*Conn, error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if conn := self.getConn(oneID, otherID); conn != nil {
		return conn, nil
	}

	one := self.getNode(oneID)
	if one == nil {
		return nil, fmt.Errorf("node %v does not exist", oneID)
	}
	other := self.getNode(otherID)
	if other == nil {
		return nil, fmt.Errorf("node %v does not exist", otherID)
	}
	conn := NewConn(self, oneID, otherID, one, other)

	label := ConnLabel(oneID, otherID)
	self.connMap[label] = len(self.Conns)
	self.Conns = append(self.Conns, conn)
	return conn, nil
}

func (self *Network) getConn(oneID, otherID string) *Conn {
	label := ConnLabel(oneID, otherID)
	i, found := self.connMap[label]
	if !found {
		return nil
	}
	return self.Conns[i]
}

// GetNodes returns the existing nodes
func (self *Network) GetNodes() []*Node {
	self.lock.Lock()
	defer self.lock.Unlock()
	return self.Nodes
}

// NewNodeWithConfig adds a new node to the network with the given config,
// returning an error if a node with the same ID or name already exists
func (self *Network) NewNodeWithConfig(conf *NodeConfig) (*Node, error) {
	self.lock.Lock()
	defer self.lock.Unlock()

	// create a random ID and PrivateKey if not set
	if conf.ID == "" {
		c := RandomNodeConfig()
		conf.ID = c.ID
		conf.PrivateKey = c.PrivateKey
	}
	id := conf.ID

	// check the node doesn't already exist
	if node := self.getNode(id); node != nil {
		return nil, fmt.Errorf("node with ID %q already exists", id)
	}

	node, err := NewNode(id, self, conf)
	if err != nil {
		return nil, err
	}

	log.Debug(fmt.Sprintf("node %v created", id))
	self.nodeMap[id] = nil
	self.Nodes = append(self.Nodes, node)

	return node, nil
}

// watchPeerEvents reads peer events from the given channel and emits
// corresponding network events
func (self *Network) watchPeerEvents(id string, eventChan <-chan *PeerEvent) {
	defer func() {
		// assume the node is now down
		self.lock.Lock()
		node := self.getNode(id)
		node.Up = false
		self.lock.Unlock()
	}()

	log.Debugf("node %s start watching event chan", id)

	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				return
			}
			peer := event.Peer
			switch event.Type {

			case PeerEventTypeAdd:
				self.DidConnect(id, peer)

			case PeerEventTypeDrop:
				self.DidDisconnect(id, peer)

			}
		}
	}
}

func (self *Network) GetNeighbours(id string) []string {
	nodes := make([]string, 0)

	for _, n := range self.Nodes {
		if self.getConn(id, n.ID) != nil {
			nodes = append(nodes, n.ID)
		}
	}

	return nodes
}
