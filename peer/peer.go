package peer

import (
	"bytes"
	"context"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	dbm "github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
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

	genesisDigest []byte

	// the following variables are managed by the process loop
	lastObservedMessage *inca.NodeMessage
	incomingPubsubCh    chan *inca.ChainPubsubMessage
	encStrat            encryption.Strategy
}

var lastObservedMessageKey = []byte("/last-observed-message")

// NewPeer builds a new peer object.
func NewPeer(
	ctx context.Context,
	le *logrus.Entry,
	db dbm.Db,
	objStore *objstore.ObjectStore,
	pubKey crypto.PubKey,
	genesisDigest []byte,
	encStrat encryption.Strategy,
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
		genesisDigest:    genesisDigest,
		incomingPubsubCh: make(chan *inca.ChainPubsubMessage, 10),
		encStrat:         encStrat,
	}
	if err := p.loadDbState(ctx); err != nil {
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

	encConf := p.encStrat.GetNodeMessageEncryptionConfigWithDigest(msgRef.GetObjectDigest())
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
	if p.lastObservedMessage != nil {
		// Rudimentary WIP checks here.
		// TODO: all of the checks, evidence collection
		lastNmTime := p.lastObservedMessage.GetTimestamp().ToTime()
		if lastNmTime.After(nmTime) {
			return errors.Errorf("old message at %s: latest %s", nmTime.String(), lastNmTime.String())
		}
	}

	innerRef := nm.GetInnerRef()
	_ = innerRef
	p.le.WithField("msg-time", nmTime.String()).Debug("received pub-sub message")
	return nil
}

// loadDbState loads the peer state from the database.
func (p *Peer) loadDbState(ctx context.Context) error {
	dat, err := p.db.Get(ctx, lastObservedMessageKey)
	if err != nil {
		return err
	}
	if dat != nil {
		// fetch last observed message from db
		p.lastObservedMessage = &inca.NodeMessage{}
		return p.objStore.GetLocal(ctx, dat, p.lastObservedMessage)
	}
	return nil
}

// ProcessNodePubsubMessage processes an incoming node pubsub message.
// The message may not be from this peer, it needs to be verfied.
func (p *Peer) ProcessNodePubsubMessage(msg *inca.ChainPubsubMessage) {
	ref := msg.GetNodeMessageRef()
	p.le.Debugf("peer pub-sub received with digest: %v", ref.GetObjectDigest())

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

// writeDbState writes the last observed message and other parameters to the db.
func (p *Peer) writeDbState() error {
	ctx := context.TODO()
	if p.lastObservedMessage != nil {
		var hash []byte
		if err := p.objStore.StoreLocal(ctx, p.lastObservedMessage, &hash, objstore.StoreParams{}); err != nil {
			return err
		}
		return p.db.Set(ctx, lastObservedMessageKey, hash)
	}
	return nil
}

// GetPublicKey returns the peer public key.
func (p *Peer) GetPublicKey() crypto.PubKey {
	return p.peerPubKey
}
