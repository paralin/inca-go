package main

import (
	"io/ioutil"
	"sync"

	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"
	"github.com/golang/protobuf/jsonpb"
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

	le := logctx.GetLogEntry(rootContext)
	sh, err := GetShell()
	if err != nil {
		return nil, err
	}

	db, err := GetDb()
	if err != nil {
		return nil, err
	}

	dat, err := ioutil.ReadFile(chainConfigPath)
	if err != nil {
		return nil, err
	}

	chainConf := &chain.Config{}
	if err := jsonpb.UnmarshalString(string(dat), chainConf); err != nil {
		return nil, err
	}

	le.Debug("loading blockchain")
	ch, err := chain.FromConfig(rootContext, db, sh, chainConf)
	if err != nil {
		return nil, err
	}
	le.WithField("chain-id", ch.GetGenesis().GetChainId()).Info("blockchain loaded")

	nod, err := node.NewNode(rootContext, db, sh, ch)
	if err != nil {
		return nil, err
	}

	nodeCached = nod
	return nod, nil
}
