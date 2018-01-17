package main

import (
	"crypto/rand"
	"io/ioutil"
	"os"

	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/node"

	"github.com/golang/protobuf/jsonpb"
	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var initChainArgs = struct {
	// ChainID is the ID for the new chain.
	ChainID string
}{}

func init() {
	incaCommands = append(incaCommands, cli.Command{
		Name:   "init",
		Usage:  "initialize a blockchain by making the genesis block",
		Action: buildProcessAction(cmdInitCluster),
		Flags: []cli.Flag{
			cli.StringFlag{
				Name:        "chain-id",
				Usage:       "the new chain id",
				Value:       initChainArgs.ChainID,
				Destination: &initChainArgs.ChainID,
			},
		},
	})
}

func cmdInitCluster(p goprocess.Process) error {
	if _, err := os.Stat(chainConfigPath); !os.IsNotExist(err) {
		return errors.Errorf("chain config already exists: %s", chainConfigPath)
	}

	le := logctx.GetLogEntry(rootContext)
	objStore, err := GetObjectStore()
	if err != nil {
		return err
	}

	le.WithField("chain-id", initChainArgs.ChainID).Debug("loading blockchain")
	ch, err := chain.NewChain(rootContext, objStore, initChainArgs.ChainID)
	if err != nil {
		return err
	}
	le.WithField("chain-id", initChainArgs.ChainID).Info("blockchain loaded")

	conf := ch.GetConfig()
	le.
		WithField("genesis-object", conf.GetGenesisRef().GetIpfs().GetObjectHash()).
		Info("successfully built genesis block")

	marshaler := &jsonpb.Marshaler{Indent: "\t"}
	dat, _ := marshaler.MarshalToString(conf)
	dat += "\n"

	if err := ioutil.WriteFile(chainConfigPath, []byte(dat), 0644); err != nil {
		return err
	}

	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return err
	}

	nodeConf := &node.Config{}
	if err := nodeConf.SetPrivKey(privKey); err != nil {
		return err
	}

	nodeConfStr, err := marshaler.MarshalToString(nodeConf)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(nodeConfigPath, []byte(nodeConfStr), 0644); err != nil {
		return err
	}

	return nil
}
