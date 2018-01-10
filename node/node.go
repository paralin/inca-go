package node

import (
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/shell"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/pbobject/ipfs"
	"github.com/jbenet/goprocess"
	// "github.com/pkg/errors"
)

// Node is an instance of an Inca p2p node.
type Node struct {
	ctx context.Context
	le  *logrus.Entry

	db     db.Db
	shell  *shell.Shell
	chain  *chain.Chain
	proc   goprocess.Process
	initCh chan chan error
}

// NewNode builds the p2p node.
// The logger can be customized with logctx.
func NewNode(
	ctx context.Context,
	db db.Db,
	ipfsShell *shell.Shell,
	chain *chain.Chain,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)

	if pbobject.GetObjectTable(ctx) == nil {
		ctx = pbobject.WithObjectTable(ctx, ipfsShell.ObjectTable.ObjectTable)
	}
	if ipfs.GetIpfsShell(ctx) == nil {
		ctx = ipfs.WithIpfsShell(ctx, ipfsShell.FileShell)
	}

	n := &Node{ctx: ctx, shell: ipfsShell, le: le, chain: chain, db: db}
	n.proc = goprocess.Go(n.processNode)
	n.initCh = make(chan chan error, 1)
	return n, nil
}

// GetProcess returns the process.
func (n *Node) GetProcess() goprocess.Process {
	return n.proc
}

// processNode is the inner process loop for the node.
func (n *Node) processNode(proc goprocess.Process) {
	// Sync up to latest known revision
	for {
		select {
		case <-n.ctx.Done():
			return
		case <-proc.Closing():
			return
		}
	}
}
