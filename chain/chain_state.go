package chain

import (
	"context"
	"math/rand"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/storageref"
)

// ChainStateSnapshot is an instance of the chain state.
type ChainStateSnapshot struct {
	// Context will be canceled when the state is invalidated.
	Context context.Context `json:"-"`
	// RoundStarted indicates if the round is in progress.
	RoundStarted bool
	// BlockRoundInfo is the current block height and round information.
	BlockRoundInfo *inca.BlockRoundInfo
	// LastBlockHeader is the last block header.
	LastBlockHeader *inca.BlockHeader
	// LastBlockRef is the reference to the last block.
	LastBlockRef *storageref.StorageRef
	// CurrentProposer contains a reference to the computed current proposer if known.
	CurrentProposer *inca.Validator
	// RoundEndTime is the time when the round will end.
	RoundEndTime time.Time
	// RoundStartTime is the time when the round will/did start.
	RoundStartTime time.Time
	// NextRoundStartTime is the time when the next round will start.
	NextRoundStartTime time.Time
	// TotalVotingPower is the current total amount of voting power.
	TotalVotingPower int32
	// ChainConfig is the current chain config.
	ChainConfig *inca.ChainConfig
	// ValidatorSet is the current validator set.
	ValidatorSet *inca.ValidatorSet
}

type chainSubscription struct {
	ch chan ChainStateSnapshot
}

// SubscribeState returns a channel and a function to cancel the subscription.
func (c *Chain) SubscribeState() (<-chan ChainStateSnapshot, func()) {
	subId := rand.Int63()
	ch := make(chan ChainStateSnapshot, 1)
	if c.lastStateSnapshot != nil {
		ch <- *c.lastStateSnapshot
	}
	c.stateSubs.Store(subId, &chainSubscription{ch: ch})
	return ch, func() { c.stateSubs.Delete(subId) }
}

// nextStateSnapshotCtx sets the next state snapshot context, canceling the old one.
func (c *Chain) nextStateSnapshotCtx() {
	if c.stateSnapshotCtxCancel != nil {
		c.stateSnapshotCtxCancel()
	}
	c.stateSnapshotCtx, c.stateSnapshotCtxCancel = context.WithCancel(c.ctx)
}

// emitNextChainState emits the next state snapshot to the subscribers.
func (c *Chain) emitNextChainState(snap *ChainStateSnapshot) {
	c.nextStateSnapshotCtx()
	snap.Context = c.stateSnapshotCtx
	c.lastStateSnapshot = snap
	c.stateSubs.Range(func(key interface{}, value interface{}) bool {
		sub := value.(*chainSubscription)
		ch := sub.ch
		val := *snap
	EnqueueLoop:
		for {
			select {
			case ch <- val:
				break EnqueueLoop
			default:
			}

			select {
			case <-ch:
			default:
			}
		}
		return true
	})
}
