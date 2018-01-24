package peer

import (
	"github.com/aperturerobotics/inca"
	"math/rand"
)

type peerSubscription struct {
	ch chan *inca.NodeMessage
}

// SubscribeMessages returns a channel and a function to cancel the subscription.
func (p *Peer) SubscribeMessages() (<-chan *inca.NodeMessage, func()) {
	subId := rand.Int63()
	ch := make(chan *inca.NodeMessage, 5)
	p.msgSubs.Store(subId, &peerSubscription{ch: ch})
	return ch, func() { p.msgSubs.Delete(subId) }
}

// emitNextMessage emits the next message to the subscribers.
func (p *Peer) emitNextNodeMessage(msg *inca.NodeMessage) {
	p.msgSubs.Range(func(key interface{}, value interface{}) bool {
		sub := value.(*peerSubscription)
		ch := sub.ch
	EnqueueLoop:
		for {
			select {
			case ch <- msg:
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
