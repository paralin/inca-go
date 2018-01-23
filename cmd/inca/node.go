package main

import (
	"io/ioutil"
	"sync"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
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

	confPath := nodeConfigPath
	if confPath == "" {
		confPath = "node_config.json"
	}

	le := logctx.GetLogEntry(rootContext)
	sh, err := GetShell()
	if err != nil {
		return nil, err
	}

	db, err := GetObjectStore()
	if err != nil {
		return nil, err
	}
	ctx := objstore.WithObjStore(rootContext, db)

	dbm, err := GetDb()
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
	ch, err := chain.FromConfig(ctx, dbm, db, chainConf)
	if err != nil {
		return nil, err
	}
	le.WithField("chain-id", ch.GetGenesis().GetChainId()).Info("blockchain loaded")
	encStrat := ch.GetEncryptionStrategy()

	{
		genEncConf := encStrat.GetGenesisEncryptionConfigWithDigest(chainConf.GetGenesisRef().GetObjectDigest())
		genCtx := pbobject.WithEncryptionConf(ctx, &genEncConf)

		gen := &inca.Genesis{}
		if err := chainConf.GetGenesisRef().FollowRef(genCtx, nil, gen); err != nil {
			return nil, err
		}

		now := time.Now()
		timeAgo := now.Sub(gen.GetTimestamp().ToTime()).String()
		le.WithField("minted-time-ago", timeAgo).Info("blockchain genesis loaded")
	}

	nodeConf := &node.Config{}
	confDat, err := ioutil.ReadFile(confPath)
	if err != nil {
		return nil, err
	}

	if err := jsonpb.UnmarshalString(string(confDat), nodeConf); err != nil {
		return nil, err
	}

	nod, err := node.NewNode(ctx, dbm, db, sh, ch, nodeConf)
	if err != nil {
		return nil, err
	}

	nodeCached = nod
	return nod, nil
}
