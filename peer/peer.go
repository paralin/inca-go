package peer

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/encryption"
	ipeer "github.com/aperturerobotics/inca/peer"

	"github.com/aperturerobotics/objstore"
	dbm "github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"

	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Peer is an observed remote node.
type Peer struct {
	nextSubId uint32
	ctx       context.Context
	le        *logrus.Entry
	// db is the inca database
	db dbm.Db
	// objStore is the object store
	objStore *objstore.ObjectStore

	// peerPubKey is the public key of the peer
	peerPubKey crypto.PubKey
	peerID     lpeer.ID

	state         ipeer.PeerState
	genesisDigest []byte
	encStrat      encryption.Strategy

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
		ctx:           ctx,
		le:            le.WithField("peer", peerID.Pretty()),
		db:            db,
		peerPubKey:    pubKey,
		peerID:        peerID,
		objStore:      objStore,
		handler:       handler,
		genesisDigest: genesisDigest,
		encStrat:      encStrat,
	}
	if err := p.readState(ctx); err != nil {
		return nil, err
	}
	return p, nil
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

	encConf := p.encStrat.GetNodeMessageEncryptionConfigWithDigest(
		p.GetPublicKey(),
		msgRef.GetObjectDigest(),
	)
	subCtx := pbobject.WithEncryptionConf(p.ctx, &encConf)

	nm := &inca.NodeMessage{}
	if err := msgRef.FollowRef(subCtx, nil, nm, nil); err != nil {
		return err
	}

	nmPubDat := nm.GetPubKey()
	if len(nmPubDat) == 0 {
		return errors.New("node message has no public key")
	}

	nmPub, err := crypto.UnmarshalPublicKey(nmPubDat)
	if err != nil {
		return errors.Wrap(err, "unmarshal node message public key")
	}

	peerPub := p.GetPublicKey()
	if !nmPub.Equals(peerPub) {
		nmPubID, _ := lpeer.IDFromPublicKey(nmPub)
		peerPubID, _ := lpeer.IDFromPublicKey(peerPub)

		return errors.Errorf(
			"node message public key mismatch: %s != %s (expected)",
			nmPubID.Pretty(),
			peerPubID.Pretty(),
		)
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
	dat, datOk, err := p.db.Get(ctx, []byte("/state"))
	if err != nil {
		return err
	}
	if !datOk {
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
	if err := p.processIncomingPubsubMessage(msg); err != nil {
		p.le.
			WithError(err).
			WithField("peer-id", msg.GetPeerId()).
			Warn("dropped message")
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
