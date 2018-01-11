package main

import (
	"github.com/urfave/cli"
)

var chainConfigPath = "chain.json"

var nodeConfigPath = "node_config.json"

func init() {
	incaFlags = append(
		incaFlags,
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
	)
}
