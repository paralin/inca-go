package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/shell"
)

var shellMtx sync.Mutex
var shellCached *shell.Shell

func init() {
	incaFlags = append(incaFlags, shell.ShellFlags...)
}

// GetShell builds / returns the shell.
func GetShell() (*shell.Shell, error) {
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

	shellCached = shell.Wrap(sh)
	// shellCached.ObjectTable.KeyResourceStore.AddKey(key []byte)
	return shellCached, nil
}
