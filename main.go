package main

import (
	"encoding/hex"
	"os"
	"path/filepath"

	"github.com/ontio/ontology/common/log"
)

func main() {
	if len(os.Args) < 2 {
		log.Error("need genesis db path")
		os.Exit(1)
	}

	chainDbPath := os.Args[1]

	// register a single vbft service
	services := map[string]simulations.ServiceFunc{
		"vbft": func(node simulations.Noder) (simulations.Service, error) {
			chainPath := filepath.Join(chainDbPath, hex.EncodeToString(node.GetID().Bytes()[:4]))
			return newVbftService(node, chainPath)
		},
	}
}
