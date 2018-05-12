package validators

import (
	"context"
	"errors"

	"github.com/aperturerobotics/inca-go/block"
)

// DenyAll denies all operations.
type DenyAll struct{}

// ValidateBlock validates a proposed block.
func (v *DenyAll) ValidateBlock(
	ctx context.Context,
	proposedBlk *block.Block,
	parentBlk *block.Block,
) (bool, error) {
	return false, errors.New("deny policy set, rejecting all block proposals")
}
