package peer

import (
	"fmt"

	dbm "github.com/aperturerobotics/inca-go/db"
	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"
)

// Peer is an observed remote node.
type Peer struct {
	// db is the inca database
	db dbm.Db
	// peerPubKey is the public key of the peer
	peerPubKey crypto.PubKey
	peerID     lpeer.ID
}

// NewPeer builds a new peer object.
func NewPeer(db dbm.Db, pubKey crypto.PubKey) *Peer {
	peerID, _ := lpeer.IDFromPublicKey(pubKey)
	db = dbm.WithPrefix(db, []byte(fmt.Sprintf("/%s", peerID.Pretty())))
	return &Peer{db: db, peerPubKey: pubKey, peerID: peerID}
}

// GetPublicKey returns the peer public key.
func (p *Peer) GetPublicKey() crypto.PubKey {
	return p.peerPubKey
}
