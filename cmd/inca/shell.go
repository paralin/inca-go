package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/shell"
	api "github.com/ipfs/go-ipfs-api"
)

var shellMtx sync.Mutex
var shellCached *api.Shell

func init() {
	incaFlags = append(incaFlags, shell.ShellFlags...)
}

// GetShell builds / returns the shell.
func GetShell() (*api.Shell, error) {
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
	// shellCached.ObjectTable.KeyResourceStore.AddKey(key []byte)
	return shellCached, nil
}
