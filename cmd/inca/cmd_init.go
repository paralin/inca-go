package main

import (
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/jbenet/goprocess"
	"github.com/urfave/cli"
)

func init() {
	incaCommands = append(incaCommands, cli.Command{
		Name:   "init",
		Usage:  "initialize a blockchain by making the genesis block",
		Action: buildProcessAction(cmdInitCluster),
	})
}

func cmdInitCluster(p goprocess.Process) error {
	le := logctx.GetLogEntry(rootContext)
	sh, err := GetShell()
	if err != nil {
		return err
	}

	_ = le
	_ = sh
	return nil
}
