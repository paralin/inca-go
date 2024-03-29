package block

import (
	"context"

	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/storageref"
)

// Proposer may propose blocks at some point during the voting window.
type Proposer interface {
	// ProposeBlock proposes a state for the next block or returns none to abstain.
	ProposeBlock(
		ctx context.Context,
		parentBlk *Block,
		chainState *state.ChainStateSnapshot,
	) (*storageref.StorageRef, error)
}
