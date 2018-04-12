package validators

import (
	"context"
	"github.com/aperturerobotics/inca-go/block"
)

// AllowAll allows all operations.
type AllowAll struct{}

// ValidateBlock validates a proposed block.
func (v *AllowAll) ValidateBlock(
	ctx *context.Context,
	proposedBlk *block.Block,
	parentBlk *block.Block,
) (bool, error) {
	return false, nil
}
