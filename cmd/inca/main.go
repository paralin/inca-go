package main

import (
	"context"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var incaCommands []cli.Command
var incaFlags []cli.Flag

func main() {
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(log)

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
