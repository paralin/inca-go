package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/shell"

	sh "github.com/ipfs/go-ipfs-api"
)

var shellMtx sync.Mutex
var shellCached *sh.Shell

func init() {
	incaFlags = append(incaFlags, shell.ShellFlags...)
}

// GetShell builds / returns the shell.
func GetShell() (*sh.Shell, error) {
	shellMtx.Lock()
	defer shellMtx.Unlock()

	if shellCached != nil {
		return shellCached, nil
	}

	le := logctx.GetLogEntry(rootContext)
	sh, err := shell.BuildCliShell(le)
	if err != nil {
		return nil, err
	}

	shellCached = sh
	return sh, nil
}
