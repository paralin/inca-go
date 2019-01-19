package main

import (
	"crypto/rand"
	"io/ioutil"
	"os"
	"sync"

	// 	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"
	"github.com/aperturerobotics/inca-go/validators"
	ichain "github.com/aperturerobotics/inca/chain"
	"github.com/golang/protobuf/jsonpb"
	"github.com/libp2p/go-libp2p-crypto"
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

	chainConf := &ichain.Config{}
	if err := jsonpb.UnmarshalString(string(dat), chainConf); err != nil {
		return nil, err
	}

	le.Debug("loading blockchain")
	ch, err := chain.FromConfig(ctx, dbm, db, chainConf, nil)
	if err != nil {
		return nil, err
	}
	le.WithField("chain-id", ch.GetGenesis().GetChainId()).Info("blockchain loaded")

	if nodeValidatorType != "" {
		nv, err := validators.GetBuiltInValidator(nodeValidatorType, ch)
		if err != nil {
			return nil, err
		}

		ch.SetBlockValidator(nv)
	}

	nodeConf := &node.Config{}
	confDat, err := ioutil.ReadFile(confPath)
	if err == nil {
		if err := jsonpb.UnmarshalString(string(confDat), nodeConf); err != nil {
			return nil, err
		}
	}

	if err != nil {
		if createInitNodeConfig && os.IsNotExist(err) {
			le.Info("minting new identity")
			privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
			if err != nil {
				return nil, err
			}

			if err := nodeConf.SetPrivKey(privKey); err != nil {
				return nil, err
			}

			jm := &jsonpb.Marshaler{Indent: "  "}
			jdat, err := jm.MarshalToString(nodeConf)
			if err != nil {
				return nil, err
			}

			if err := ioutil.WriteFile(confPath, []byte(jdat), 0644); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	nodPriv, err := nodeConf.UnmarshalPrivKey()
	if err != nil {
		return nil, err
	}

	nod, err := node.NewNode(ctx, dbm, db, ch, nodPriv)
	if err != nil {
		return nil, err
	}

	nodeCached = nod
	return nod, nil
}
