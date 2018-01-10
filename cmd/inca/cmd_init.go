package main

import (
	"io/ioutil"
	"os"

	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"

	"github.com/golang/protobuf/jsonpb"
	"github.com/jbenet/goprocess"
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
	sh, err := GetShell()
	if err != nil {
		return err
	}

	db, err := GetDb()
	if err != nil {
		return err
	}

	le.WithField("chain-id", initChainArgs.ChainID).Debug("loading blockchain")
	ch, err := chain.NewChain(rootContext, db, sh, initChainArgs.ChainID)
	if err != nil {
		return err
	}
	le.WithField("chain-id", initChainArgs.ChainID).Info("blockchain loaded")

	conf := ch.GetConfig()
	le.
		WithField("genesis-object", conf.GetGenesisRef().GetIpfs().GetObjectHash()).
		Info("successfully built genesis block")

	dat, _ := (&jsonpb.Marshaler{Indent: "\t"}).MarshalToString(conf)
	dat += "\n"

	return ioutil.WriteFile(chainConfigPath, []byte(dat), 0644)
}
