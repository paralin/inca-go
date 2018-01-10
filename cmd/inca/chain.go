package main

import (
	"github.com/urfave/cli"
)

var chainConfigPath = "chain.json"

func init() {
	incaFlags = append(
		incaFlags,
		cli.StringFlag{
			Name:        "chain-config-path",
			Usage:       "the new chain config will be saved to `PATH`",
			Value:       chainConfigPath,
			Destination: &chainConfigPath,
		},
	)
}
