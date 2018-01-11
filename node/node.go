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

	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	// "github.com/pkg/errors"
)

// Node is an instance of an Inca p2p node.
type Node struct {
	ctx context.Context
	le  *logrus.Entry

	db       db.Db
	shell    *shell.Shell
	chain    *chain.Chain
	proc     goprocess.Process
	initCh   chan chan error
	privKey  crypto.PrivKey
	nodeAddr lpeer.ID
}

// NewNode builds the p2p node.
// The logger can be customized with logctx.
func NewNode(
	ctx context.Context,
	db db.Db,
	ipfsShell *shell.Shell,
	chain *chain.Chain,
	config *Config,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)

	if pbobject.GetObjectTable(ctx) == nil {
		ctx = pbobject.WithObjectTable(ctx, ipfsShell.ObjectTable.ObjectTable)
	}
	if ipfs.GetObjectShell(ctx) == nil {
		ctx = ipfs.WithObjectShell(ctx, ipfsShell.FileShell)
	}

	privKey, err := config.UnmarshalPrivKey()
	if err != nil {
		return nil, err
	}

	n := &Node{ctx: ctx, shell: ipfsShell, le: le, chain: chain, db: db, privKey: privKey}
	n.nodeAddr, err = lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

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
	n.le.WithField("addr", n.nodeAddr.Pretty()).Info("node running")
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
