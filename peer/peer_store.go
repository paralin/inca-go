package peer

import (
	"sync"

	"github.com/aperturerobotics/inca-go/db"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
)

// PeerStore manages peer objects
type PeerStore struct {
	db    db.Db
	peers sync.Map // map[lpeer.ID]*Peer
}

// NewPeerStore builds a new peer store, loading the initial set from the db.
func NewPeerStore(dbm db.Db) *PeerStore {
	dbm = db.WithPrefix(dbm, []byte("/peers"))
	return &PeerStore{db: dbm}
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
func (ps *PeerStore) GetPeerWithPubKey(pubKey crypto.PubKey) *Peer {
	id, _ := lpeer.IDFromPublicKey(pubKey)
	if peer := ps.GetPeer(id); peer != nil {
		return peer
	}

	p := NewPeer(ps.db, pubKey)
	peerObj, loaded := ps.peers.LoadOrStore(id, p)
	if loaded {
		p = peerObj.(*Peer)
	}

	return p
}
