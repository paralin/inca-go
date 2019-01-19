package main

import (
	"context"
	"sync"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
)

// Chat implements the mempool Application interface.
type Chat struct {
	// ctx is the context
	ctx context.Context
	// dbm contains local state.
	dbm db.Db

	// stateMtx guards state.
	stateMtx sync.Mutex
	// state is the application state.
	state ChatState
}

// NewChat constructs a new chat instance.
// State is loaded from the database.
func NewChat(ctx context.Context, localDb db.Db) (*Chat, error) {
	c := &Chat{dbm: localDb, ctx: ctx}
	if err := c.readState(ctx); err != nil {
		return nil, err
	}

	return c, nil
}

// GetStateAtBlock returns a state handle at the desired block.
// The block state object is decoded for convenience.
// This should return a unique snapshot / handle for each call.
func (c *Chat) GetStateAtBlock(
	ctx context.Context,
	blk *block.Block,
	blkState *transaction.BlockState,
) (mempool.ApplicationState, error) {
	return c.getVirtualState(ctx, blkState.GetApplicationStateRef())
}

// _ is a type assertion
var _ mempool.Application = ((*Chat)(nil))
