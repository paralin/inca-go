package node

import (
	"context"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
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

	db        db.Db
	objStore  *objstore.ObjectStore
	shell     *api.Shell
	proposer  *chain.Proposer
	chain     *chain.Chain
	proc      goprocess.Process
	initCh    chan chan error
	privKey   crypto.PrivKey
	nodeAddr  lpeer.ID
	peerStore *peer.PeerStore

	chainSub *api.PubSubSubscription
}

// NewNode builds the p2p node.
// The logger can be customized with logctx.
func NewNode(
	ctx context.Context,
	db db.Db,
	objStore *objstore.ObjectStore,
	shell *api.Shell,
	ch *chain.Chain,
	config *Config,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)
	privKey, err := config.UnmarshalPrivKey()
	if err != nil {
		return nil, err
	}

	genesisRef := ch.GetGenesisRef()
	peerStore := peer.NewPeerStore(ctx, db, objStore, genesisRef.GetObjectDigest())

	n := &Node{
		ctx:       ctx,
		shell:     shell,
		le:        le,
		chain:     ch,
		db:        db,
		privKey:   privKey,
		objStore:  objStore,
		peerStore: peerStore,
		proposer:  chain.NewProposer(le, ch),
	}
	n.nodeAddr, err = lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	// start listening on pubsub
	if err := n.initPubSub(); err != nil {
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
	ctx := n.ctx
	n.le.WithField("addr", n.nodeAddr.Pretty()).Info("node running")

	errCh := make(chan error, 5)
	go func() {
		errCh <- n.proposer.ManageProposer(ctx)
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case <-proc.Closing():
			return
		case err := <-errCh:
			n.le.WithError(err).Fatal("internal error")
			return
		}
	}
}
