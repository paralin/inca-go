package main

import (
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
	nod, err := GetNode()
	if err != nil {
		return err
	}
	p.AddChild(nod.GetProcess())

	<-nod.GetProcess().Closing()
	return nil
}
