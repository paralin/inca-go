package chain

import (
	"context"
	"math/rand"

	"github.com/aperturerobotics/inca-go/chain/state"
)

type chainSubscription struct {
	ch chan state.ChainStateSnapshot
}

// SubscribeState returns a channel and a function to cancel the subscription.
func (c *Chain) SubscribeState() (<-chan state.ChainStateSnapshot, func()) {
	subID := rand.Int63()
	ch := make(chan state.ChainStateSnapshot, 1)
	if c.lastStateSnapshot != nil {
		ch <- *c.lastStateSnapshot
	}

	c.stateSubs.Store(subID, &chainSubscription{ch: ch})
	return ch, func() { c.stateSubs.Delete(subID) }
}

// nextStateSnapshotCtx sets the next state snapshot context, canceling the old one.
func (c *Chain) nextStateSnapshotCtx() {
	if c.stateSnapshotCtxCancel != nil {
		c.stateSnapshotCtxCancel()
	}
	c.stateSnapshotCtx, c.stateSnapshotCtxCancel = context.WithCancel(c.ctx)
}

// emitNextChainState emits the next state snapshot to the subscribers.
func (c *Chain) emitNextChainState(snap *state.ChainStateSnapshot) {
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
