package block

// Validator validates proposed blocks as a voting member.
type Validator interface {
	// ValidateBlock validates a proposed block.
	ValidateBlock(proposedBlk *Block, parentBlk *Block) error
}
