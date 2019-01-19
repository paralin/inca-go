package mempool

import (
	"context"

	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/utils/transaction"
)

// Application manages state and transaction verification and processing.
type Application interface {
	// GetStateAtBlock returns a state handle at the desired block.
	// The block state object is decoded for convenience.
	// This should return a unique snapshot / handle for each call.
	GetStateAtBlock(
		ctx context.Context,
		blk *block.Block,
		blkState *transaction.BlockState,
	) (ApplicationState, error)
}

// ApplicationState represents a reference to a pending application state.
// This state can be modified when building a proposed block or validating blocks.
type ApplicationState interface {
	// PrePoolCheckTx checks a transaction before placing it into the pool.
	// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
	// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
	PrePoolCheckTx(ctx context.Context, tx *transaction.Transaction) (sysErr bool, err error)
	// Apply applies a transaction to the state.
	// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
	// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
	Apply(ctx context.Context, tx *transaction.Transaction) (sysErr bool, txErr error)
	// Serialize returns a state reference that can be used in a block.
	// If the state is already serialized, should return the same ref.
	Serialize(ctx context.Context) (*cid.BlockRef, error)
}
