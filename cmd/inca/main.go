package main

import (
	"context"
	"os"

	"github.com/Sirupsen/logrus"
	_ "github.com/aperturerobotics/inca-go/encryption/all"
	"github.com/aperturerobotics/inca-go/logctx"
	_ "github.com/aperturerobotics/storageref/all"
	"github.com/urfave/cli"
)

var rootContext context.Context
var incaCommands []cli.Command
var incaFlags []cli.Flag

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(log)
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	rootContext = logctx.WithLogEntry(ctx, le)

	app := cli.NewApp()
	app.Name = "inca"
	app.Usage = "inca utilities cli"
	app.HideVersion = true
	app.Commands = incaCommands
	app.Flags = incaFlags
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err.Error())
	}
}
