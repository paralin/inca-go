package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/node"
)

var nodeMtx sync.Mutex
var nodeCached *node.Node

// GetNode builds / returns the cli node.
func GetNode() (*node.Node, error) {
	nodeMtx.Lock()
	defer nodeMtx.Unlock()

	if nodeCached != nil {
		return nodeCached, nil
	}

	sh, err := GetShell()
	if err != nil {
		return nil, err
	}

	nod, err := node.NewNode(rootContext, sh)
	if err != nil {
		return nil, err
	}

	nodeCached = nod
	return nod, nil
}
