package mempool

import (
	"context"

	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/storageref"
)

// Application manages state and transaction verification and processing.
type Application interface {
	// GetState returns a handle to the application state.
	GetState() ApplicationState
	// Promote sets a application state as the new primary state.
	Promote(state ApplicationState)
}

// ApplicationState represents a reference to the current application state.
type ApplicationState interface {
	// Snapshot returns a copy of the application state for proposing transactions.
	Snapshot() ApplicationState
	// Serialize returns a state reference that can be used in a block.
	Serialize() (*storageref.StorageRef, error)
	// PrePoolCheckTx checks a transaction before placing it into the pool.
	// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
	// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
	PrePoolCheckTx(ctx context.Context, tx *transaction.Transaction) (sysErr bool, err error)
	// Apply applies a transaction to the state.
	// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
	// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
	Apply(ctx context.Context, tx *transaction.Transaction) (sysErr bool, txErr error)
}
