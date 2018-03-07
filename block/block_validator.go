package block

import (
	"context"

	"github.com/aperturerobotics/storageref"
)

// Validator validates proposed blocks as a voting member.
type Validator interface {
	// ValidateBlock validates a proposed block.
	ValidateBlock(proposedBlk *Block, parentBlk *Block) error
}

// Proposer proposes blocks at some point during the voting window.
type Proposer interface {
	// ProposeBlock proposes a state for the next block or returns none to abstain.
	ProposeBlock(ctx context.Context, parentBlk *Block) (*storageref.StorageRef, error)
}
