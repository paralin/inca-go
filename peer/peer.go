package peer

import (
	"context"
	"sync"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	ipeer "github.com/aperturerobotics/inca/peer"

	hblock "github.com/aperturerobotics/hydra/block"
	"github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/cid"
	dbm "github.com/aperturerobotics/hydra/object"

	lpeer "github.com/aperturerobotics/bifrost/peer"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Peer is an observed remote node.
type Peer struct {
	nextSubId  uint32
	ctx        context.Context
	le         *logrus.Entry
	store      dbm.ObjectStore
	rootCursor *object.Cursor

	// peerPubKey is the public key of the peer
	peerPubKey crypto.PubKey
	peerID     lpeer.ID

	state      ipeer.PeerState
	genesisRef *cid.BlockRef

	msgSubs sync.Map
	handler PeerHandler
}

// PeerHandler handles peer events.
type PeerHandler interface {
	// HandleBlockCommit handles an incoming block.
	HandleBlockCommit(p *Peer, blockRef *cid.BlockRef, block *inca.Block) error
	// HandleAppMessage handles an application node message.
	HandleAppMessage(p *Peer, nodeMessage *inca.NodeMessage, nodeMessageRef *cid.BlockRef) error
}

// NewPeer builds a new peer object.
func NewPeer(
	ctx context.Context,
	le *logrus.Entry,
	rootCursor *object.Cursor,
	store dbm.ObjectStore,
	pubKey crypto.PubKey,
	genesisRef *cid.BlockRef,
	handler PeerHandler,
) (*Peer, error) {
	peerID, _ := lpeer.IDFromPublicKey(pubKey)
	p := &Peer{
		ctx:        ctx,
		le:         le.WithField("peer", peerID.Pretty()),
		store:      dbm.NewPrefixer(store, peerID.Pretty()+"/"),
		peerPubKey: pubKey,
		peerID:     peerID,
		handler:    handler,
		rootCursor: rootCursor,
		genesisRef: genesisRef,
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
	msgDat, err := msgRef.MarshalKey()
	if err != nil {
		return err
	}
	if len(msgDat) == 0 {
		return errors.New("object digest cannot be empty")
	}

	_, msgCursor := p.rootCursor.BuildTransactionAtRef(nil, msgRef)
	nmi, err := msgCursor.Unmarshal(func() hblock.Block {
		return &inca.NodeMessage{}
	})
	nm := nmi.(*inca.NodeMessage)

	/*
		nmPeerIdStr := nm.GetPeerId()
		if len(nmPeerIdStr) == 0 {
			return errors.New("node message has no peer id")
		}

		nmPeerId, err := lpeer.IDB58Decode(nmPeerIdStr)
		if err != nil {
			return errors.Wrap(err, "parse node message peer id")
		}

		if !nmPeerId.MatchesPublicKey(p.peerPubKey) {
			peerPubID, _ := lpeer.IDFromPublicKey(p.peerPubKey)

			return errors.Errorf(
				"node message public key mismatch: %s != %s (expected)",
				nmPeerId.Pretty(),
				peerPubID.Pretty(),
			)
		}
	*/

	return p.processIncomingNodeMessage(nm, msgRef)
}

// processIncomingNodeMessage processes an incoming node message.
func (p *Peer) processIncomingNodeMessage(nm *inca.NodeMessage, nmRef *cid.BlockRef) error {
	if !p.genesisRef.EqualsRef(nm.GetGenesisRef()) {
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
		_, blkCursor := p.rootCursor.BuildTransactionAtRef(nil, nm.GetInnerRef())
		blk, err := block.GetBlock(p.ctx, blkCursor, p.store)
		if err != nil {
			return err
		}

		err = p.handler.HandleBlockCommit(p, nm.GetInnerRef(), blk.GetInnerBlock())
		if err != nil {
			le.WithError(err).Warn("block commit handler failed")
		}
	} else if nm.GetMessageType() == inca.NodeMessageType_NodeMessageType_APP {
		if err := p.handler.HandleAppMessage(p, nm, nmRef); err != nil {
			le.WithError(err).Warn("app message handler failed")
		}
	}

	p.emitNextNodeMessage(nm)
	return nil
}

// readState loads the peer state from the database.
func (p *Peer) readState(ctx context.Context) error {
	dat, datOk, err := p.store.GetObject("state")
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

	return p.store.SetObject("state", dat)
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
