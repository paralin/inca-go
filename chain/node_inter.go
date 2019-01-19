package chain

import (
	"context"

	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/peer"
)

// Node interacts with the network.
type Node interface {
	// SendMessage submits a message to the node pubsub.
	SendMessage(
		ctx context.Context,
		msgType inca.NodeMessageType,
		appMsgType uint32,
		msgInnerRef *cid.BlockRef,
	) error
	// GetPeerStore returns the peer store from the node.
	GetPeerStore() *peer.PeerStore
}
