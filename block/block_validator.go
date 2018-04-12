package block

import "context"

// Validator validates proposed blocks as a voting member.
type Validator interface {
	// ValidateBlock validates a proposed block.
	ValidateBlock(
		// ctx is the context to use for validating.
		ctx context.Context,
		// proposedBlk is the next block in the chain, that we are validating.
		proposedBlk *Block,
		// parentBlk is the parent block, if known, may be nil.
		parentBlk *Block,
	) (
		// markValid indicates this block should be trusted as part of the chain.
		// returning false will mark the block as disjointed, unless the parent block is valid.
		markValid bool,
		// err indicates there was some issue with this block.
		err error,
	)
}
