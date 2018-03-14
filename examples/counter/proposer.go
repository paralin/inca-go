package main

import (
	"context"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/pkg/errors"
)

// Proposer is the counter proposer.
type Proposer struct {
	encConf pbobject.EncryptionConfig
}

// ProposeBlock proposes a state for the next block or returns none to abstain.
func (p *Proposer) ProposeBlock(
	ctx context.Context,
	parentBlkInner *block.Block,
) (*storageref.StorageRef, error) {
	ctx = pbobject.WithEncryptionConf(ctx, &p.encConf)
	objStore := objstore.GetObjStore(ctx)
	if objStore == nil {
		return nil, errors.New("object store not set, cannot propose blocks")
	}

	parentBlk := &Block{Block: parentBlkInner}
	parentState, err := parentBlk.GetState(ctx)
	if err != nil {
		return nil, err
	}

	le := logctx.GetLogEntry(ctx)
	nextState := &State{CounterVal: parentState.GetCounterVal() + 1}
	le.
		WithField("counter", parentState.GetCounterVal()).
		WithField("counter-next", nextState.GetCounterVal()).
		Info("proposing counter inc")

	propRef, _, err := objStore.StoreObject(ctx, nextState, p.encConf)
	return propRef, err
}
