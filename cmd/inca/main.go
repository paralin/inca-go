package main

import (
	"context"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/urfave/cli"
)

var rootContext context.Context
var incaCommands []cli.Command
var incaFlags []cli.Flag

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	ctx, ctxCancel := context.WithCancel(context.Background())
	rootContext = logctx.WithLogEntry(ctx, logrus.NewEntry(log))
	defer ctxCancel()

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
