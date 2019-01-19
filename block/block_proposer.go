package block

import (
	"context"

	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca-go/chain/state"
)

// Proposer may propose blocks at some point during the voting window.
type Proposer interface {
	// ProposeBlock proposes a state for the next block or returns none to abstain.
	ProposeBlock(
		ctx context.Context,
		parentBlk *Block,
		chainState *state.ChainStateSnapshot,
	) (*cid.BlockRef, error)
}
