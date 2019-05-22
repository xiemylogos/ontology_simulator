package peer

import (
	"github.com/ontio/ontology-crypto/keypair"
	types "github.com/ontio/ontology/consensus/vbft/config"
)

// NodeConfig is the configuration used to start a node in a simulation
// network
type NodeConfig struct {
	// ID is the node's ID which is used to identify the node in the
	// simulation network
	ID string
	// PrivateKey is the node's private key which is used by the devp2p
	// stack to encrypt communications
	PrivateKey keypair.PrivateKey `json:"private_key"`

	// Services are the names of the services which should be run when
	// starting the node (for SimNodes it should be the names of services
	// contained in SimAdapter.services, for other nodes it should be
	// services registered by calling the RegisterService function)
	Services []string
}

type Msg struct {
	One      string `json:"one"`
	Other    string `json:"other"`
	Protocol string `json:"protocol"`
	Code     uint64 `json:"code"`
	Received bool   `json:"received"`

	Payload []byte
}

func PubkeyID(pub keypair.PublicKey) string {
	return types.PubkeyID(pub)
}

func Pubkey(nodeid string) (keypair.PublicKey, error) {
	return types.Pubkey(nodeid)
}

func RandomNodeConfig() *NodeConfig {
	sk, pk, err := keypair.GenerateKeyPair(keypair.PK_ECDSA, keypair.P256)
	if err != nil {
		return nil
	}
	id := PubkeyID(&pk)
	return &NodeConfig{
		ID:         id,
		PrivateKey: sk,
	}
}
