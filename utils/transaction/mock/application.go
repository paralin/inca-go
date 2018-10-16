package mock

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
)

// Application implements a mock application.
type Application struct {
}

// GetStateAtBlock returns a state handle at the desired block.
// The block state object is decoded for convenience.
// This should return a unique snapshot / handle for each call.
func (a *Application) GetStateAtBlock(
	ctx context.Context,
	blk *block.Block,
	blkState *transaction.BlockState,
) (mempool.ApplicationState, error) {
	vs := &VirtualState{}
	state := &vs.state
	err := blkState.
		GetApplicationStateRef().
		FollowRef(ctx, nil, state, nil)
	fmt.Printf("looking up app state: %v %v\n", blkState.
		GetApplicationStateRef().
		GetObjectDigest(), err)
	if err != nil {
		return nil, err
	}

	return vs, nil
}

// _ is a type assertion.
var _ mempool.Application = ((*Application)(nil))
