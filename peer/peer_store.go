package peer

import (
	"context"
	"sync"

	lpeer "github.com/aperturerobotics/bifrost/peer"
	"github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/cid"
	dbm "github.com/aperturerobotics/hydra/object"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/libp2p/go-libp2p-crypto"
)

// PeerStore manages peer objects
type PeerStore struct {
	ctx        context.Context
	store      dbm.ObjectStore
	peers      sync.Map // map[lpeer.ID]*Peer
	genesisRef *cid.BlockRef
	handler    PeerHandler
	rootCursor *object.Cursor
}

// NewPeerStore builds a new peer store, loading the initial set from the db.
func NewPeerStore(
	ctx context.Context,
	rootCursor *object.Cursor,
	store dbm.ObjectStore,
	genesisRef *cid.BlockRef,
	handler PeerHandler,
) *PeerStore {
	store = dbm.NewPrefixer(store, "peers/")
	return &PeerStore{
		ctx:        ctx,
		store:      store,
		genesisRef: genesisRef,
		handler:    handler,
		rootCursor: rootCursor,
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
	p, err := NewPeer(
		ps.ctx,
		le,
		ps.rootCursor,
		ps.store,
		pubKey,
		ps.genesisRef,
		ps.handler,
	)
	if err != nil {
		return nil, err
	}

	peerObj, loaded := ps.peers.LoadOrStore(id, p)
	if loaded {
		p = peerObj.(*Peer)
	}
	return p, nil
}
