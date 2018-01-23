package chain

import (
	"context"
	"fmt"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/libp2p/go-libp2p-peer"
)

// Proposer controls proposing new blocks on a Chain.
type Proposer struct {
	ctx   context.Context
	state ProposerState
	ch    *Chain
	le    *logrus.Entry
	dbm   db.Db

	proposeMtx sync.Mutex
}

// NewProposer builds a new Proposer.
func NewProposer(ctx context.Context, privKey crypto.PrivKey, dbm db.Db, ch *Chain) (*Proposer, error) {
	le := logctx.GetLogEntry(ctx)
	pid, err := peer.IDFromPrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	dbm = db.WithPrefix(dbm, []byte(fmt.Sprintf("/proposer/%s", pid.Pretty())))
	p := &Proposer{ctx: ctx, ch: ch, le: le, dbm: dbm}
	if err := p.readState(ctx); err != nil {
		return nil, err
	}
	if err := p.writeState(ctx); err != nil {
		return nil, err
	}

	return p, nil
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (p *Proposer) readState(ctx context.Context) error {
	dat, err := p.dbm.Get(ctx, []byte("/state"))
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return nil
	}

	return proto.Unmarshal(dat, &p.state)
}

// writeState writes the state to the database.
func (p *Proposer) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&p.state)
	if err != nil {
		return err
	}

	return p.dbm.Set(ctx, []byte("/state"), dat)
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
