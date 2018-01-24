package chain

import (
	"github.com/aperturerobotics/inca"
)

// BlockCommitRatio is the ratio of voting power committed to not committed to finalize a block.
const BlockCommitRatio float32 = 0.66

// Block is a block header wrapped with some context.
type Block struct {
	*inca.BlockHeader
}
