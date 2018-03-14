package main

import (
	"context"
	"crypto/rand"
	"io/ioutil"
	"os"

	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/cmd/inca/validators"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"
	"github.com/aperturerobotics/inca-go/shell"
	ichain "github.com/aperturerobotics/inca/chain"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/ipfs"
	"github.com/aperturerobotics/objstore/localdb"

	"github.com/golang/protobuf/jsonpb"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	// _ imports all encryption types
	_ "github.com/aperturerobotics/objectenc/all"
	// _ imports all storage reference types
	_ "github.com/aperturerobotics/storageref/all"
	// _ imports all encryption strategies
	_ "github.com/aperturerobotics/inca-go/encryption/all"
)

var rootContext context.Context
var createInitNodeConfig bool
var nodeValidatorType string
var chainConfigPath = "chain.json"
var nodeConfigPath = "node_config.json"

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(log)

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	ctx = logctx.WithLogEntry(ctx, le)
	rootContext = ctx

	app := cli.NewApp()
	app.Name = "counter"
	app.Usage = "inca counter example"
	app.HideVersion = true
	app.Action = runCounterExample
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:        "init-node-config",
			Usage:       "If set, node_config.json will be created if it doesn't exist.",
			Destination: &createInitNodeConfig,
		},
		cli.StringFlag{
			Name:        "validator",
			Usage:       "select a blockchain validator implementation: none, allow, deny",
			Destination: &nodeValidatorType,
			Value:       "none",
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
	app.Before = func(c *cli.Context) error {
		if nodeConfigPath == "" {
			nodeConfigPath = "node_config.json"
		}

		return nil
	}

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err.Error())
	}
}

func runCounterExample(c *cli.Context) error {
	var validator validators.BuiltInValidator
	if nodeValidatorType != "" {
		nv, err := validators.GetBuiltInValidator(nodeValidatorType)
		if err != nil {
			return err
		}

		validator = nv
	}

	ctx := rootContext
	le := logctx.GetLogEntry(ctx)

	sh, err := shell.BuildCliShell(le)
	if err != nil {
		return err
	}

	dbm, err := db.BuildCliDb(le)
	if err != nil {
		return err
	}

	localStore := localdb.NewLocalDb(dbm)
	remoteStore := ipfs.NewRemoteStore(sh)
	objStore := objstore.NewObjectStore(ctx, localStore, remoteStore)

	ctx = objstore.WithObjStore(ctx, objStore)

	nodeConf := &node.Config{}
	confDat, err := ioutil.ReadFile(nodeConfigPath)
	if err == nil {
		if err := jsonpb.UnmarshalString(string(confDat), nodeConf); err != nil {
			return err
		}
	}
	if err != nil {
		if createInitNodeConfig && os.IsNotExist(err) {
			le.Info("minting new identity")
			privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
			if err != nil {
				return err
			}

			if err := nodeConf.SetPrivKey(privKey); err != nil {
				return err
			}

			jm := &jsonpb.Marshaler{Indent: "  "}
			jdat, err := jm.MarshalToString(nodeConf)
			if err != nil {
				return err
			}

			if err := ioutil.WriteFile(nodeConfigPath, []byte(jdat), 0644); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	chainProposer := &Proposer{}
	chainConf := &ichain.Config{}
	nodePriv, err := nodeConf.UnmarshalPrivKey()
	if err != nil {
		return err
	}

	{
		if _, err := os.Stat(chainConfigPath); os.IsNotExist(err) {
			le.Info("minting new blockchain")
			mintCtx, mintCtxCancel := context.WithCancel(ctx)
			nch, err := chain.NewChain(
				mintCtx,
				dbm,
				objStore,
				"counter-test-1",
				nodePriv,
				validator,
				chainProposer,
			)
			mintCtxCancel()
			if err != nil {
				return err
			}

			chainConf = nch.GetConfig()
			chainConfStr, err := (&jsonpb.Marshaler{}).MarshalToString(chainConf)
			if err != nil {
				return err
			}

			if err := ioutil.WriteFile(chainConfigPath, []byte(chainConfStr), 0644); err != nil {
				return err
			}
		} else {
			dat, err := ioutil.ReadFile(chainConfigPath)
			if err != nil {
				return err
			}

			if err := jsonpb.UnmarshalString(string(dat), chainConf); err != nil {
				return err
			}
		}
	}

	le.Info("loading blockchain")
	proposer := &Proposer{}
	ch, err := chain.FromConfig(ctx, dbm, objStore, chainConf, validator, proposer)
	if err != nil {
		return err
	}
	le.WithField("chain-id", ch.GetGenesis().GetChainId()).Info("blockchain loaded")

	le.Info("starting node")
	nod, err := node.NewNode(ctx, dbm, objStore, sh, ch, nodeConf)
	if err != nil {
		return err
	}

	<-nod.GetProcess().Closed()
	return nil
}
