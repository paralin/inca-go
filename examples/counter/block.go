package main

import (
	"context"

	"github.com/aperturerobotics/inca-go/block"
)

// Block is a counter block.
type Block struct {
	*block.Block
}

// GetState returns the counter value from the state.
func (b *Block) GetState(ctx context.Context) (*State, error) {
	blockState := &State{}

	stateRef := b.GetStateRef()
	if !stateRef.IsEmpty() {
		if err := stateRef.FollowRef(ctx, nil, blockState); err != nil {
			return nil, err
		}
	}

	return blockState, nil
}
