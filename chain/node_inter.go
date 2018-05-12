package chain

import (
	"context"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/peer"
	"github.com/aperturerobotics/storageref"
)

// Node interacts with the network.
type Node interface {
	// SendMessage submits a message to the node pubsub.
	SendMessage(
		ctx context.Context,
		msgType inca.NodeMessageType,
		msgInnerRef *storageref.StorageRef,
	) error
	// GetPeerStore returns the peer store from the node.
	GetPeerStore() *peer.PeerStore
}
