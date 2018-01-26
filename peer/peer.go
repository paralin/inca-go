package peer

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	dbm "github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
)

// Peer is an observed remote node.
type Peer struct {
	ctx context.Context
	le  *logrus.Entry
	// db is the inca database
	db dbm.Db
	// objStore is the object store
	objStore *objstore.ObjectStore

	// peerPubKey is the public key of the peer
	peerPubKey crypto.PubKey
	peerID     lpeer.ID

	state         PeerState
	genesisDigest []byte

	// the following variables are managed by the process loop
	incomingPubsubCh chan *inca.ChainPubsubMessage
	encStrat         encryption.Strategy

	msgSubs sync.Map
	handler PeerHandler
}

// PeerHandler handles peer events.
type PeerHandler interface {
	// HandleBlockCommit handles an incoming block.
	HandleBlockCommit(p *Peer, blockRef *storageref.StorageRef, block *inca.Block) error
}

// NewPeer builds a new peer object.
func NewPeer(
	ctx context.Context,
	le *logrus.Entry,
	db dbm.Db,
	objStore *objstore.ObjectStore,
	pubKey crypto.PubKey,
	genesisDigest []byte,
	encStrat encryption.Strategy,
	handler PeerHandler,
) (*Peer, error) {
	peerID, _ := lpeer.IDFromPublicKey(pubKey)
	db = dbm.WithPrefix(db, []byte(fmt.Sprintf("/%s", peerID.Pretty())))
	p := &Peer{
		ctx:              ctx,
		le:               le.WithField("peer", peerID.Pretty()),
		db:               db,
		peerPubKey:       pubKey,
		peerID:           peerID,
		objStore:         objStore,
		handler:          handler,
		genesisDigest:    genesisDigest,
		incomingPubsubCh: make(chan *inca.ChainPubsubMessage, 10),
		encStrat:         encStrat,
	}
	if err := p.readState(ctx); err != nil {
		return nil, err
	}
	go p.processPeer()
	return p, nil
}

// processPeer processes a peer.
func (p *Peer) processPeer() {
	for {
		select {
		case <-p.ctx.Done():
			return
		case pubSubMsg := <-p.incomingPubsubCh:
			if err := p.processIncomingPubsubMessage(pubSubMsg); err != nil {
				p.le.WithError(err).Warn("dropping invalid pubsub message")
			}
		}
	}
}

// processIncomingPubsubMessage processes an incoming pubsub message.
func (p *Peer) processIncomingPubsubMessage(pubsubMsg *inca.ChainPubsubMessage) error {
	if p.peerID.Pretty() != pubsubMsg.GetPeerId() {
		return errors.New("pub-sub message not from this peer")
	}

	msgRef := pubsubMsg.GetNodeMessageRef()
	if len(msgRef.GetObjectDigest()) == 0 {
		return errors.New("object digest cannot be empty")
	}

	encConf := p.encStrat.GetNodeMessageEncryptionConfigWithDigest(p.GetPublicKey(), msgRef.GetObjectDigest())
	subCtx := pbobject.WithEncryptionConf(p.ctx, &encConf)

	nm := &inca.NodeMessage{}
	if err := msgRef.FollowRef(subCtx, nil, nm); err != nil {
		return err
	}

	return p.processIncomingNodeMessage(nm)
}

// processIncomingNodeMessage processes an incoming node message.
func (p *Peer) processIncomingNodeMessage(nm *inca.NodeMessage) error {
	if bytes.Compare(nm.GetGenesisRef().GetObjectDigest(), p.genesisDigest) != 0 {
		return errors.New("genesis reference mismatch")
	}

	if err := nm.GetTimestamp().Validate(); err != nil {
		return errors.WithMessage(err, "invalid timestamp")
	}

	nmTime := nm.GetTimestamp().ToTime()
	le := p.le.
		WithField("msg-time", nmTime.String())
	if p.state.LastObservedMessage != nil {
		// Rudimentary WIP checks here.
		// TODO: all of the checks, evidence collection
		lastNmTime := p.state.LastObservedMessage.GetTimestamp().ToTime()
		if lastNmTime.After(nmTime) {
			return errors.Errorf("old message at %s: latest %s", nmTime.String(), lastNmTime.String())
		}
		le = le.WithField("last-msg-time", lastNmTime.String())
	}

	innerRef := nm.GetInnerRef()
	_ = innerRef
	le.WithField("msg-type", nm.GetMessageType().String()).Debug("received node message")
	p.state.LastObservedMessage = nm
	if err := p.writeState(p.ctx); err != nil {
		le.WithError(err).Warn("cannot write peer state")
	}

	now := time.Now()
	if nmTime.After(now) {
		return errors.Errorf("message is in the future by %s", nmTime.Sub(now).String())
	}
	if now.Sub(nmTime) > (time.Second * 120) {
		le.Debug("ignoring too-old message (older than 2 minutes)")
		return nil
	}

	if nm.GetMessageType() == inca.NodeMessageType_NodeMessageType_BLOCK_COMMIT {
		blk, err := block.FollowBlockRef(p.ctx, nm.GetInnerRef(), p.encStrat)
		if err != nil {
			return err
		}

		err = p.handler.HandleBlockCommit(p, nm.GetInnerRef(), blk)
		if err != nil {
			le.WithError(err).Warn("block commit handler failed")
		}
	}

	p.emitNextNodeMessage(nm)
	return nil
}

// readState loads the peer state from the database.
func (p *Peer) readState(ctx context.Context) error {
	dat, err := p.db.Get(ctx, []byte("/state"))
	if err != nil {
		return err
	}
	if dat == nil {
		return nil
	}
	return proto.Unmarshal(dat, &p.state)
}

// writeState writes the last observed message and other parameters to the db.
func (p *Peer) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&p.state)
	if err != nil {
		return err
	}

	return p.db.Set(ctx, []byte("/state"), dat)
}

// ProcessNodePubsubMessage processes an incoming node pubsub message.
// The message may not be from this peer, it needs to be verfied.
func (p *Peer) ProcessNodePubsubMessage(msg *inca.ChainPubsubMessage) {
EnqueueLoop:
	for {
		select {
		case p.incomingPubsubCh <- msg:
			break EnqueueLoop
		default:
			select {
			case <-p.incomingPubsubCh:
				p.le.Warn("dropping old node message")
			default:
			}
		}
	}
}

// GetPublicKey returns the peer public key.
func (p *Peer) GetPublicKey() crypto.PubKey {
	return p.peerPubKey
}

// GetPeerID returns the peer id.
func (p *Peer) GetPeerID() lpeer.ID {
	return p.peerID
}
