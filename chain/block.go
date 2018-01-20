package chain

import (
	"github.com/aperturerobotics/inca"
)

// Block is a block header wrapped with some context.
type Block struct {
	*inca.BlockHeader
}
