package node

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"

	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"

	"github.com/golang/protobuf/proto"
	api "github.com/ipfs/go-ipfs-api"
	"github.com/jbenet/goprocess"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/sirupsen/logrus"
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
	validator *chain.Validator
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
	privKey crypto.PrivKey,
) (*Node, error) {
	le := logctx.GetLogEntry(ctx)
	genesisRef := ch.GetGenesisRef()
	nodeAddr, err := lpeer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	n := &Node{
		ctx:      ctx,
		shell:    shell,
		le:       le,
		chain:    ch,
		db:       db.WithPrefix(dbm, []byte(fmt.Sprintf("/node/%s", nodeAddr.Pretty()))),
		privKey:  privKey,
		objStore: objStore,
		nodeAddr: nodeAddr,
		outboxCh: make(chan *inca.NodeMessage),
	}

	n.peerStore = peer.NewPeerStore(
		ctx,
		dbm,
		objStore,
		genesisRef.GetObjectDigest(),
		ch.GetEncryptionStrategy(),
		n.chain,
	)

	n.proposer, err = chain.NewProposer(ctx, privKey, dbm, ch, n, n.peerStore)
	if err != nil {
		return nil, err
	}

	n.validator, err = chain.NewValidator(ctx, privKey, dbm, ch, n, n.peerStore)
	if err != nil {
		return nil, err
	}

	if _, err := n.peerStore.GetPeerWithPubKey(privKey.GetPublic()); err != nil {
		return nil, err
	}

	if err := n.readState(); err != nil {
		return nil, err
	}

	if err := n.readGenesisState(ctx); err != nil {
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

// GetPeerStore returns the peer store.
func (n *Node) GetPeerStore() *peer.PeerStore {
	return n.peerStore
}

// GetChain returns the chain object.
func (n *Node) GetChain() *chain.Chain {
	return n.chain
}

// SendMessage submits a message to the node pubsub.
func (n *Node) SendMessage(
	ctx context.Context,
	msgType inca.NodeMessageType,
	appMsgType uint32,
	msgInnerRef *storageref.StorageRef,
) error {
	// Build the NodeMessage object
	ts := timestamp.Now()
	nm := &inca.NodeMessage{
		MessageType:    msgType,
		GenesisRef:     n.chain.GetGenesisRef(),
		InnerRef:       msgInnerRef,
		Timestamp:      &ts,
		AppMessageType: appMsgType,
	}

	select {
	case n.outboxCh <- nm:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// readGenesisState attempts to follow the genesis block references.
func (n *Node) readGenesisState(ctx context.Context) error {
	encStrat := n.chain.GetEncryptionStrategy()

	gen := &inca.Genesis{}
	genRef := n.chain.GetGenesisRef()
	{
		genEncConf := encStrat.GetGenesisEncryptionConfigWithDigest(genRef.GetObjectDigest())
		genCtx := pbobject.WithEncryptionConf(ctx, &genEncConf)

		if err := genRef.FollowRef(genCtx, nil, gen, nil); err != nil {
			return err
		}

		now := time.Now()
		timeAgo := now.Sub(gen.GetTimestamp().ToTime()).String()
		n.le.WithField("minted-time-ago", timeAgo).Info("blockchain genesis loaded")
	}

	initChainConfig := &inca.ChainConfig{}
	{
		confRef := gen.GetInitChainConfigRef()
		encConf := encStrat.GetBlockEncryptionConfigWithDigest(confRef.GetObjectDigest())
		subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		if err := confRef.FollowRef(subCtx, nil, initChainConfig, nil); err != nil {
			return err
		}
	}

	// Assert that all validators have peers
	initValidatorSet := &inca.ValidatorSet{}
	{
		validatorSetRef := initChainConfig.GetValidatorSetRef()
		encConf := encStrat.GetBlockEncryptionConfigWithDigest(validatorSetRef.GetObjectDigest())
		subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		if err := validatorSetRef.FollowRef(subCtx, nil, initValidatorSet, nil); err != nil {
			return err
		}
	}

	for _, validator := range initValidatorSet.GetValidators() {
		validatorPub, err := validator.ParsePublicKey()
		if err != nil {
			return err
		}

		if _, err := n.peerStore.GetPeerWithPubKey(validatorPub); err != nil {
			return err
		}
	}

	return nil
}

// readState attempts to read the state out of the db.
func (n *Node) readState() error {
	dat, datOk, err := n.db.Get(n.ctx, []byte("/state"))
	if err != nil || !datOk {
		return err
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

// pubsubSendMsg actually emits a node message to the pubsub channel.
func (n *Node) pubsubSendMsg(msg *inca.NodeMessage) error {
	tsNow := timestamp.Now()
	msg.PrevMsgRef = n.state.LastMsgRef
	msg.Timestamp = &tsNow

	msgSref, _, err := n.objStore.StoreObject(
		n.ctx,
		msg,
		n.chain.GetEncryptionStrategy().GetNodeMessageEncryptionConfig(n.privKey),
	)
	if err != nil {
		return err
	}

	pubSubMsg := &inca.ChainPubsubMessage{
		NodeMessageRef: msgSref,
		PeerId:         n.nodeAddr.Pretty(),
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

	n.le.WithField("msg-type", msg.GetMessageType().String()).Debug("sent node message")
	return nil
}

// processPubSub processes the pub-sub channel.
func (n *Node) processPubSub() error {
	defer n.chainSub.Cancel()

	for {
		record, err := n.chainSub.Next()
		if err != nil {
			return err
		}

		le := n.le.WithField("ipfs-peer-id", record.From().Pretty())
		msg := &inca.ChainPubsubMessage{}
		dat, err := base64.StdEncoding.DecodeString(string(record.Data()))
		if err != nil {
			le.WithError(err).Warn("pub-sub message invalid base64 string")
			continue
		}

		if err := proto.Unmarshal(dat, msg); err != nil {
			le.WithError(err).Warn("pub-sub message invalid storage ref object")
			continue
		}

		peerId, err := lpeer.IDB58Decode(msg.GetPeerId())
		if err != nil {
			le.WithError(err).Warn("pub-sub message ignoring invalid peer id")
			continue
		}

		le = le.WithField("peer-id", peerId.Pretty())
		peer := n.peerStore.GetPeer(peerId)
		if peer == nil {
			le.WithError(err).Warn("pub-sub message ignoring unknown peer id")
			continue
		}

		peer.ProcessNodePubsubMessage(msg)
	}
}

// processNode is the inner process loop for the node.
func (n *Node) processNode(proc goprocess.Process) {
	ctx := n.ctx
	n.le.WithField("addr", n.nodeAddr.Pretty()).Info("node running")

	errCh := make(chan error, 5)
	go func() {
		errCh <- n.proposer.ManageProposer(ctx)
	}()
	go func() {
		errCh <- n.validator.ManageValidator(ctx)
	}()
	go func() {
		errCh <- n.processPubSub()
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
