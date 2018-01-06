package node

import (
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/jbenet/goprocess"
	// "github.com/pkg/errors"

	sh "github.com/ipfs/go-ipfs-api"
)

// Node is an instance of an Inca p2p node.
type Node struct {
	ctx context.Context
	le  *logrus.Entry

	shell  *sh.Shell
	proc   goprocess.Process
	initCh chan chan error
}

// NewNode builds the p2p node.
// The logger can be customized with logctx.
func NewNode(
	ctx context.Context,
	ipfsShell *sh.Shell,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)
	n := &Node{ctx: ctx, shell: ipfsShell, le: le}
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
	for {
		select {
		case <-n.ctx.Done():
			return
		}
	}
}
