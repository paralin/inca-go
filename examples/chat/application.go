package main

import (
	"sync"

	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
	"github.com/aperturerobotics/objstore/db"
)

// Chat implements the mempool Application interface.
type Chat struct {
	// dbm contains local state.
	dbm db.Db

	// stateMtx guards state.
	stateMtx sync.Mutex
	// state is the application state.
	state mempool.ApplicationState
}

// NewChat constructs a new chat instance.
// State is loaded from the database.
func NewChat(localDb db.Db) (*Chat, error) {
	// TODO: loadState, validate
	return &Chat{dbm: localDb}, nil
}

// GetState returns a handle to the application state.
func (c *Chat) GetState() mempool.ApplicationState {
	c.stateMtx.Lock()
	defer c.stateMtx.Unlock()

	return c.state
}

// Promote sets a application state as the new primary state.
func (c *Chat) Promote(state mempool.ApplicationState) {
	c.stateMtx.Lock()
	defer c.stateMtx.Unlock()

	c.state = state
}

// _ is a type assertion
var _ mempool.Application = ((*Chat)(nil))
