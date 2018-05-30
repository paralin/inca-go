package mempool

import (
	"context"
	"time"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/storageref"
	"github.com/pkg/errors"
)

// Proposer is a block proposer that pulls transactions from a mempool.
type Proposer struct {
	opts    ProposerOpts
	mempool *Mempool
}

// NewProposer builds a new proposer.
func NewProposer(
	memPool *Mempool,
	opts ProposerOpts,
) (*Proposer, error) {
	return &Proposer{opts: opts, mempool: memPool}, nil
}

// ProposerOpts are options passed to the mempool proposer.
type ProposerOpts struct {
	// ProposeDeadlineRatio is the max percentage of round duration to use for gathering transactions.
	// If this is set to 1, the proposal will be fired after the wait ratio.
	// If this is set to 0, a default of 0.5 is used.
	ProposeDeadlineRatio float32
	// ProposeWaitRatio is the min percentage of round duration to use for gathering transactions.
	// If this is set to 0, the proposal will be fired after a single transaction.
	// If this is set to 1, a default of 0.15 is used.
	ProposeWaitRatio float32
}

// Validate validates the data.
func (o *ProposerOpts) Validate() error {
	if o.ProposeDeadlineRatio > 1 || o.ProposeDeadlineRatio < 0 {
		return errors.Errorf("propose deadline ratio out of range (0, 1): %v", o.ProposeDeadlineRatio)
	}

	if o.ProposeWaitRatio > 1 || o.ProposeWaitRatio < 0 {
		return errors.Errorf("propose wait ratio out of range (0, 1): %v", o.ProposeWaitRatio)
	}

	return nil
}

// ProposeBlock proposes a state for the next block or returns none to abstain.
func (p *Proposer) ProposeBlock(
	ctx context.Context,
	parentBlk *block.Block,
	chainState *state.ChainStateSnapshot,
) (*storageref.StorageRef, error) {
	// minWaitCh fires after the minimum wait time.
	var minWaitCh <-chan time.Time
	// maxWaitCh fires after the maximum wait time.
	var maxWaitCh <-chan time.Time

	_ = minWaitCh
	_ = maxWaitCh
	deadlineRatio := p.opts.ProposeDeadlineRatio
	if deadlineRatio == 0 {
		// fire after half the proposal time
		deadlineRatio = 0.5
	} else if deadlineRatio == 1 {
		// fire after wait ratio
	}

	return nil, nil
}
