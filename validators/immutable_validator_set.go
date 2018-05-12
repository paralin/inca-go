package validators

import (
	"context"

	// 	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain"
	// 	"github.com/aperturerobotics/inca-go/chain/state"
	// 	"github.com/aperturerobotics/pbobject"
	// 	"github.com/aperturerobotics/pbobject"
)

// ImmutableValidatorSet disallows any changes to validator set.
type ImmutableValidatorSet struct {
	ch *chain.Chain
}

// NewImmutableValidatorSet builds a new immutable validator set implementation.
func NewImmutableValidatorSet(ch *chain.Chain) block.Validator {
	return &ImmutableValidatorSet{ch: ch}
}

// ValidateBlock validates a proposed block.
func (v *ImmutableValidatorSet) ValidateBlock(
	ctx context.Context,
	proposedBlk *block.Block,
	parentBlk *block.Block,
) (isValid bool, err error) {
	// need parent to verify vset
	// OR, can check it is equivalent to the last known block set (TODO)
	if parentBlk == nil {
		return false, nil
	}

	/*
		chainConfig := &inca.ChainConfig{}
		confRef := parentBlk.GetHeader().GetChainConfigRef()
		subCtx := pbobject.WithEncryptionConf(ctx, v.ch.GetEncryptionStrategy())
		confRef.FollowRef(ctx, nil, chainConfig)
	*/

	/*
		var chainConf inca.ChainConfig
			subCtx :=
			err = v.ch.GetGenesis().GetInitChainConfigRef().FollowRef(ctx, nil, &chainConf)
			if err != nil {
				return
			}

			vsetRef := chainConf.GetValidatorSetRef()
			var validatorSet inca.ValidatorSet
			err = vsetRef.FollowRef(ctx, nil, out pbobject.Object)
	*/
	return false, nil
}
