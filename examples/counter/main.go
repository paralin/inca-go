package main

import (
	"context"
	"os"

	"github.com/aperturerobotics/inca-go/examples/common"
	"github.com/aperturerobotics/inca-go/validators"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"

	// _ imports all encryption types
	_ "github.com/aperturerobotics/objectenc/all"
	// _ imports all storage reference types
	_ "github.com/aperturerobotics/storageref/all"
	// _ imports all encryption strategies
	_ "github.com/aperturerobotics/inca-go/encryption/all"
)

var createInitNodeConfig bool
var nodeValidatorType string
var chainConfigPath = "chain.json"
var nodeConfigPath = "node_config.json"

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)

	app := cli.NewApp()
	app.Name = "counter"
	app.Usage = "inca counter example"
	app.HideVersion = true
	app.Action = runCounterExample
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:        "validator",
			Usage:       "select a blockchain validator implementation: none, allow, deny",
			Destination: &nodeValidatorType,
			Value:       "none",
		},
	}
	app.Flags = append(app.Flags, common.Flags...)

	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err.Error())
	}
}

func runCounterExample(c *cli.Context) error {
	chainProposer := &Proposer{}
	conf, err := common.Build(context.Background(), chainProposer, nil, nil)
	if err != nil {
		return err
	}

	/*
		ctx := conf.Context
		le := conf.LogEntry
	*/

	nod := conf.Node
	ch := nod.GetChain()

	if nodeValidatorType != "" {
		chainValidator, err := validators.GetBuiltInValidator(nodeValidatorType, ch)
		if err != nil {
			return err
		}
		ch.SetBlockValidator(chainValidator)
	}

	<-nod.GetProcess().Closed()

	return nil
}
