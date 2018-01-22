package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-peer"
)

// Proposer controls proposing new blocks on a Chain.
type Proposer struct {
	ch  *Chain
	le  *logrus.Entry
	dbm db.Db

	proposeMtx sync.Mutex
}

// NewProposer builds a new Proposer.
func NewProposer(le *logrus.Entry, privKey crypto.PrivKey, dbm db.Db, ch *Chain) (*Proposer, error) {
	pid, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	dbm = db.WithPrefix(dbm, []byte(fmt.Sprintf("/proposer/%s", pid.Pretty())))
	return &Proposer{ch: ch, le: le, dbm: dbm}, nil
}

// ManageProposer manages the proposer lifecycle.
func (p *Proposer) ManageProposer(ctx context.Context) error {
	p.le.Debug("proposer running")
	defer p.le.Debug("proposer shut down")

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
