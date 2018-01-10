package main

import (
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/jbenet/goprocess"
	"github.com/urfave/cli"
)

func init() {
	incaCommands = append(incaCommands, cli.Command{
		Name:   "node",
		Usage:  "run a light node",
		Action: buildProcessAction(cmdLightNode),
	})
}

func cmdLightNode(p goprocess.Process) error {
	le := logctx.GetLogEntry(rootContext)
	nod, err := GetNode()
	if err != nil {
		return err
	}
	p.AddChild(nod.GetProcess())

	le.Info("node built and running")
	<-nod.GetProcess().Closing()
	return nil
}
