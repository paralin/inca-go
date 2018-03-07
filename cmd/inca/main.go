package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/urfave/cli"

	// _ imports all encryption types
	_ "github.com/aperturerobotics/objectenc/all"
	// _ imports all storage reference types
	_ "github.com/aperturerobotics/storageref/all"
	// _ imports all encryption strategies
	_ "github.com/aperturerobotics/inca-go/encryption/all"
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
