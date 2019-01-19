// Package common implements common parts of the Inca examples.
package common

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"

	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"
	ichain "github.com/aperturerobotics/inca/chain"

	"github.com/golang/protobuf/jsonpb"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var createInitNodeConfig bool
var chainConfigPath = "chain.json"
var nodeConfigPath = "node_config.json"

// Flags is the set of common CLI flags.
var Flags = []cli.Flag{
	cli.BoolFlag{
		Name:        "init-node-config",
		Usage:       "If set, node_config.json will be created if it doesn't exist.",
		Destination: &createInitNodeConfig,
	},
	cli.StringFlag{
		Name:        "chain-config-path",
		Usage:       "the path to the chain config",
		Value:       chainConfigPath,
		Destination: &chainConfigPath,
	},
	cli.StringFlag{
		Name:        "node-config-path",
		Usage:       "the path to the node config",
		Value:       nodeConfigPath,
		Destination: &nodeConfigPath,
	},
}

// Common contains the output of the initialization function.
type Common struct {
	// Context is the root context.
	// It is configured with the object store.
	Context context.Context
	// LogEntry is the log entry.
	LogEntry *logrus.Entry
	// LocalStore is the local database.
	LocalStore *localdb.LocalDb
	// LocalDbm stores local information about the system.
	LocalDbm db.Db
	// RemoteStore is the remote store.
	RemoteStore *ipfs.RemoteStore
	// ObjStore layers localstore on remotestore.
	ObjStore *objstore.ObjectStore
	// Node is the instantiated node.
	Node *node.Node
	// NodeConfig contains the node configuration.
	NodeConfig *node.Config
	// BlockProposer contains the block proposer.
	BlockProposer block.Proposer
	// BlockValidator contains the block validator.
	BlockValidator block.Validator
}

// Build reads the CLI arguments and builds the common object.
func Build(
	ctx context.Context,
	proposer block.Proposer,
	validator block.Validator,
	buildInitState func(ctx context.Context) (*cid.BlockRef, error),
) (*Common, error) {
	le := logctx.GetLogEntry(ctx)

	sh, err := shell.BuildCliShell(le)
	if err != nil {
		return nil, err
	}

	dbm, err := dbcli.BuildCliDb(le)
	if err != nil {
		return nil, err
	}

	localStore := localdb.NewLocalDb(dbm)
	remoteStore := ipfs.NewRemoteStore(sh)
	objStore := objstore.NewObjectStore(ctx, localStore, remoteStore)

	ctx = objstore.WithObjStore(ctx, objStore)

	nodeConf := &node.Config{}
	confDat, err := ioutil.ReadFile(nodeConfigPath)
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

			if err := ioutil.WriteFile(nodeConfigPath, []byte(jdat), 0644); err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	chainConf := &ichain.Config{}
	nodePriv, err := nodeConf.UnmarshalPrivKey()
	if err != nil {
		return nil, err
	}

	{
		if _, err := os.Stat(chainConfigPath); os.IsNotExist(err) {
			le.Info("minting new blockchain")
			var initState *cid.BlockRef
			if buildInitState != nil {
				initState, err = buildInitState(ctx)
				if err != nil {
					return nil, err
				}
			}
			mintCtx, mintCtxCancel := context.WithCancel(ctx)
			nch, err := chain.NewChain(
				mintCtx,
				dbm,
				objStore,
				"counter-test-1",
				nodePriv,
				nil,
				initState,
			)
			mintCtxCancel()
			if err != nil {
				return nil, err
			}
			nch.SetBlockProposer(proposer)

			chainConf = nch.GetConfig()
			chainConfStr, err := (&jsonpb.Marshaler{}).MarshalToString(chainConf)
			if err != nil {
				return nil, err
			}

			if err := ioutil.WriteFile(chainConfigPath, []byte(chainConfStr), 0644); err != nil {
				return nil, err
			}
		} else {
			dat, err := ioutil.ReadFile(chainConfigPath)
			if err != nil {
				return nil, err
			}

			if err := jsonpb.UnmarshalString(string(dat), chainConf); err != nil {
				return nil, err
			}
		}
	}

	le.Info("loading blockchain")
	ch, err := chain.FromConfig(ctx, dbm, objStore, chainConf, nil)
	if err != nil {
		return nil, err
	}
	ch.SetBlockProposer(proposer)
	le.WithField("chain-id", ch.GetGenesis().GetChainId()).Info("blockchain loaded")

	le.Info("starting node")
	nod, err := node.NewNode(ctx, dbm, objStore, sh, ch, nodePriv)
	if err != nil {
		return nil, err
	}

	return &Common{
		Context:        ctx,
		LogEntry:       le,
		LocalStore:     localStore,
		LocalDbm:       dbm,
		RemoteStore:    remoteStore,
		ObjStore:       objStore,
		Node:           nod,
		NodeConfig:     nodeConf,
		BlockProposer:  proposer,
		BlockValidator: validator,
	}, nil
}
