package validators

import (
	"github.com/aperturerobotics/inca-go/block"
)

// AllowValidator allows all operations.
type AllowValidator struct{}

// ValidateBlock validates a proposed block.
func (v *AllowValidator) ValidateBlock(
	proposedBlk *block.Block,
	parentBlk *block.Block,
) error {
	return nil
}
