package peer

import (
	"context"

	"github.com/aperturerobotics/inca-go/db"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
)

// Peer is an observed remote node.
type Peer struct {
	// db is the inca database
	db db.Db
	// peerPubKey is the public key of the peer
	peerPubKey crypto.PubKey
	peerID lpeer.ID
}

// NewPeer builds a new peer object, loading known state from the DB.
func NewPeer(ctx context.Context, db db.Db, pubKey crypto.PubKey) (*Peer, error) {
	peerID, err := 
	return &Peer{db: db, peerPubKey: pubKey, peerID: }
}

// GetPublicKey returns the peer public key.
func (p *Peer) GetPublicKey() crypto.PubKey {
	return p.peerPubKey
}
