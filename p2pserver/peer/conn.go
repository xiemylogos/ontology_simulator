package peer

import (
	"fmt"

	"github.com/ontio/ontology/common/log"
)

// Conn represents a connection between two nodes in the network
type Conn struct {
	One   string `json:"one"`
	Other string `json:"other"`

	network *Network
	Up      bool `json:"up"`

	one   *Node
	other *Node
}

func NewConn(network *Network, oneID, otherID string, one, other *Node) *Conn {
	return &Conn{
		network: network,
		One:     oneID,
		Other:   otherID,
		one:     one,
		other:   other,
	}
}

// nodesUp returns whether both nodes are currently up
func (self *Conn) nodesUp() error {
	if !self.one.Up {
		return fmt.Errorf("one %v is not up", self.One)
	}
	if !self.other.Up {
		return fmt.Errorf("other %v is not up", self.Other)
	}
	return nil
}

// String returns a log-friendly string
func (self *Conn) String() string {
	return fmt.Sprintf("Conn %v->%v", self.One, self.Other)
}

func (self *Conn) TurnUp() error {
	self.Up = true
	self.one.PeerConnected(self)
	self.other.PeerConnected(self)

	log.Debugf("connection up: one: %s, other: %s", self.One, self.Other)

	return nil
}

func (self *Conn) TurnDown() error {
	self.Up = false
	self.one.PeerDisconnected(self)
	self.other.PeerDisconnected(self)
	return nil
}

// ConnLabel generates a deterministic string which represents a connection
// between two nodes, used to compare if two connections are between the same
// nodes
func ConnLabel(source, target string) string {
	var first, second string
	if first != second {
		first = target
		second = source
	} else {
		first = source
		second = target
	}
	return fmt.Sprintf("%v-%v", first, second)
}
