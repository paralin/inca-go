package node

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
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
	state     NodeState
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
	outboxCh chan *inca.NodeMessage
}

// NewNode builds the p2p node.
// The logger can be customized with logctx.
func NewNode(
	ctx context.Context,
	dbm db.Db,
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
	peerStore := peer.NewPeerStore(ctx, dbm, objStore, genesisRef.GetObjectDigest())

	nodeAddr, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	n := &Node{
		ctx:       ctx,
		shell:     shell,
		le:        le,
		chain:     ch,
		db:        db.WithPrefix(dbm, []byte(fmt.Sprintf("/node/%s", nodeAddr.Pretty()))),
		privKey:   privKey,
		objStore:  objStore,
		peerStore: peerStore,
		nodeAddr:  nodeAddr,
		outboxCh:  make(chan *inca.NodeMessage),
	}

	n.proposer, err = chain.NewProposer(ctx, privKey, dbm, ch)
	if err != nil {
		return nil, err
	}

	if err := n.readState(); err != nil {
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

// readState attempts to read the state out of the db.
func (n *Node) readState() error {
	dat, err := n.db.Get(n.ctx, []byte("/state"))
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return nil
	}

	return proto.Unmarshal(dat, &n.state)
}

// writeState writes the state to the database.
func (n *Node) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&n.state)
	if err != nil {
		return err
	}

	return n.db.Set(ctx, []byte("/state"), dat)
}

// GetProcess returns the process.
func (n *Node) GetProcess() goprocess.Process {
	return n.proc
}

// SendMessage submits a message to the node pubsub.
func (n *Node) SendMessage(ctx context.Context, msgType inca.NodeMessageType, msgInner pbobject.Object) error {
	// Build the NodeMessage object
	nm := &inca.NodeMessage{
		MessageType: msgType,
		GenesisRef:  n.chain.GetGenesisRef(),
	}
	sref, _, err := n.objStore.StoreObject(
		ctx,
		nm,
		n.chain.GetEncryptionStrategy().GetNodeMessageEncryptionConfig(),
	)
	if err != nil {
		return err
	}
	nm.InnerRef = sref

	select {
	case n.outboxCh <- nm:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// pubsubSendMsg actually emits a node message to the pubsub channel.
func (n *Node) pubsubSendMsg(msg *inca.NodeMessage) error {
	tsNow := timestamp.Now()
	msg.PrevMsgRef = n.state.LastMsgRef
	msg.Timestamp = &tsNow

	msgSref, _, err := n.objStore.StoreObject(
		n.ctx,
		msg,
		n.chain.GetEncryptionStrategy().GetNodeMessageEncryptionConfig(),
	)
	if err != nil {
		return err
	}

	pubSubMsg := &inca.ChainPubsubMessage{
		NodeMessageRef: msgSref,
	}

	dat, err := proto.Marshal(pubSubMsg)
	if err != nil {
		return err
	}

	n.state.LastMsgRef = msgSref
	if err := n.writeState(n.ctx); err != nil {
		return err
	}

	datb64 := base64.StdEncoding.EncodeToString(dat)
	if err := n.shell.PubSubPublish(n.chain.GetPubsubTopic(), datb64); err != nil {
		return err
	}

	return nil
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
		case msg := <-n.outboxCh:
			if err := n.pubsubSendMsg(msg); err != nil {
				n.le.WithError(err).Warn("unable to emit pubsub message")
			}
		}
	}
}
