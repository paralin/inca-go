package main

import (
	"github.com/jbenet/goprocess"
	"github.com/urfave/cli"
)

var createInitNodeConfig bool
var nodeValidatorType string

func init() {
	incaCommands = append(incaCommands, cli.Command{
		Name:  "node",
		Usage: "run a node",
		Flags: []cli.Flag{
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
		},
		Action: buildProcessAction(cmdNode),
	})
}

func cmdNode(p goprocess.Process) error {
	nod, err := GetNode()
	if err != nil {
		return err
	}
	p.AddChild(nod.GetProcess())

	<-nod.GetProcess().Closing()
	return nil
}
