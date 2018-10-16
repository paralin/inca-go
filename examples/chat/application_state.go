package main

import (
	"context"

	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/golang/protobuf/proto"
)

// ChatVirtualState is a state handle that can be edited in memory and serialized.
type ChatVirtualState struct {
	state ChatState
}

// getVirtualState returns the chat virtual state at the reference.
func (c *Chat) getVirtualState(
	ctx context.Context,
	appState *storageref.StorageRef,
) (*ChatVirtualState, error) {
	vs := &ChatVirtualState{}
	state := &vs.state
	if err := appState.FollowRef(ctx, nil, state, nil); err != nil {
		return nil, err
	}

	return vs, nil
}

// PrePoolCheckTx checks a transaction before placing it into the pool.
// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
func (s *ChatVirtualState) PrePoolCheckTx(
	ctx context.Context,
	tx *transaction.Transaction,
) (sysErr bool, err error) {
	if _, err := GetTransaction(ctx, tx); err != nil {
		return false, err
	}

	// TODO: further validation
	return false, nil
}

// Apply applies a transaction to the state.
// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
func (s *ChatVirtualState) Apply(
	ctx context.Context,
	tx *transaction.Transaction,
) (sysErr bool, txErr error) {
	txd, err := GetTransaction(ctx, tx)
	if err != nil {
		return false, err
	}

	// TODO: further validation
	_ = txd

	s.state.MessageCount++
	return false, nil
}

// Serialize returns a state reference that can be used in a block.
// If the state is already serialized, should return the same ref.
func (s *ChatVirtualState) Serialize(ctx context.Context) (*storageref.StorageRef, error) {
	encConf := pbobject.GetEncryptionConf(ctx)
	if encConf == nil {
		encConf = &pbobject.EncryptionConfig{}
	}

	objStore := objstore.GetObjStore(ctx)
	sref, _, err := objStore.StoreObject(ctx, &s.state, *encConf)
	return sref, err
}

// readState loads the application state from the database.
func (c *Chat) readState(ctx context.Context) error {
	dat, datOk, err := c.dbm.Get(ctx, []byte("/chat-state"))
	if err != nil || !datOk {
		return err
	}

	return proto.Unmarshal(dat, &c.state)
}

// GetObjectTypeID returns the object type string, used to identify types.
func (c *ChatState) GetObjectTypeID() *pbobject.ObjectTypeID {
	return &pbobject.ObjectTypeID{
		TypeUuid: "/inca/example/chat/app-state",
	}
}
