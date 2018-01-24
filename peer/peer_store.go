package peer

import (
	"context"
	"sync"

	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
)

// PeerStore manages peer objects
type PeerStore struct {
	ctx           context.Context
	db            db.Db
	peers         sync.Map // map[lpeer.ID]*Peer
	objStore      *objstore.ObjectStore
	genesisDigest []byte
	encStrat      encryption.Strategy
	handler       PeerHandler
}

// NewPeerStore builds a new peer store, loading the initial set from the db.
func NewPeerStore(
	ctx context.Context,
	dbm db.Db,
	objStore *objstore.ObjectStore,
	genesisDigest []byte,
	encStrat encryption.Strategy,
	handler PeerHandler,
) *PeerStore {
	dbm = db.WithPrefix(dbm, []byte("/peers"))
	return &PeerStore{
		ctx:           ctx,
		db:            dbm,
		objStore:      objStore,
		genesisDigest: genesisDigest,
		encStrat:      encStrat,
		handler:       handler,
	}
}

// GetPeer gets a peer from the store.
func (ps *PeerStore) GetPeer(peerID lpeer.ID) *Peer {
	peerObj, ok := ps.peers.Load(peerID)
	if !ok {
		return nil
	}
	return peerObj.(*Peer)
}

// GetPeerWithPubKey builds a peer with a public key if it doesn't already exist.
func (ps *PeerStore) GetPeerWithPubKey(pubKey crypto.PubKey) (*Peer, error) {
	id, _ := lpeer.IDFromPublicKey(pubKey)
	if peer := ps.GetPeer(id); peer != nil {
		return peer, nil
	}

	le := logctx.GetLogEntry(ps.ctx)
	p, err := NewPeer(ps.ctx, le, ps.db, ps.objStore, pubKey, ps.genesisDigest, ps.encStrat, ps.handler)
	if err != nil {
		return nil, err
	}
	peerObj, loaded := ps.peers.LoadOrStore(id, p)
	if loaded {
		p = peerObj.(*Peer)
	}

	return p, nil
}
