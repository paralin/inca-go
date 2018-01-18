package node

import (
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore"
	"github.com/jbenet/goprocess"

	api "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	// "github.com/pkg/errors"
)

// Node is an instance of an Inca p2p node.
type Node struct {
	ctx context.Context
	le  *logrus.Entry

	db       db.Db
	objStore *objstore.ObjectStore
	shell    *api.Shell
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
	objStore *objstore.ObjectStore,
	shell *api.Shell,
	chain *chain.Chain,
	config *Config,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)
	privKey, err := config.UnmarshalPrivKey()
	if err != nil {
		return nil, err
	}

	n := &Node{
		ctx:      ctx,
		shell:    shell,
		le:       le,
		chain:    chain,
		db:       db,
		privKey:  privKey,
		objStore: objStore,
	}
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
