package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/jbenet/goprocess"
	"github.com/urfave/cli"
)

// buildProcessAction builds a CLI action from a process.
func buildProcessAction(f func(goprocess.Process) error) cli.ActionFunc {
	return func(c *cli.Context) error {
		le := logctx.GetLogEntry(rootContext)
		p := goprocess.Go(func(p goprocess.Process) {
			errCh := make(chan error, 1)
			p.SetTeardown(func() error {
				return <-errCh
			})
			errCh <- f(p)
		})

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		select {
		case <-p.Closing():
		case <-sigs:
		}

		le.Info("shutting down")
		return p.Close()
	}
}
