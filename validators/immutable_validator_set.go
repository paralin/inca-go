package validators

import (
	"context"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain"
)

// ImmutableValidatorSet disallows any changes to validator set, but trusts the 
type ImmutableValidatorSet struct{
	ch *chain.Chain
}

// NewImmutableValidatorSet builds a new immutable validator set implementation.
func NewImmutableValidatorSet(ch *chain.Chain) *ImmutableValidatorSet{
	return &ImmutableValidatorSet{ch: ch}
}

// ValidateBlock validates a proposed block.
func (v *ImmutableValidatorSet) ValidateBlock(
	ctx context.Context,
	proposedBlk *block.Block,
	parentBlk *block.Block,
) (isValid bool, err error) {
	var chainConf inca.ChainConfig
	subCtx := 
	err = v.ch.GetGenesis().GetInitChainConfigRef().FollowRef(ctx, nil, &chainConf)
	if err != nil {
		return
	}

	vsetRef := chainConf.GetValidatorSetRef()
	var validatorSet inca.ValidatorSet
	err = vsetRef.FollowRef(ctx, nil, out pbobject.Object)
}
