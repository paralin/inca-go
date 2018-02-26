package validators

import (
	"errors"

	"github.com/aperturerobotics/inca-go/block"
)

// DenyValidator denies all operations.
type DenyValidator struct{}

// ValidateBlock validates a proposed block.
func (v *DenyValidator) ValidateBlock(
	proposedBlk *block.Block,
	parentBlk *block.Block,
) error {
	return errors.New("deny policy set, rejecting all block proposals")
}
