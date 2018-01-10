package main

import (
	"context"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/objtable"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/pbobject/ipfs"
	"github.com/urfave/cli"
)

var rootContext context.Context
var incaCommands []cli.Command
var incaFlags []cli.Flag
var objTable = objtable.NewObjectTable()

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(log)
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	rootContext = logctx.WithLogEntry(ctx, le)
	rootContext = pbobject.WithObjectTable(rootContext, objTable.ObjectTable)

	app := cli.NewApp()
	app.Name = "inca"
	app.Usage = "inca utilities cli"
	app.HideVersion = true
	app.Commands = incaCommands
	app.Flags = incaFlags
	app.Before = func(c *cli.Context) error {
		sh, err := GetShell()
		if err != nil {
			return err
		}
		rootContext = ipfs.WithIpfsShell(rootContext, sh.FileShell)
		return nil
	}
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err.Error())
	}
}
