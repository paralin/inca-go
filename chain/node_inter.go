package chain

import (
	"context"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/storageref"
)

// NodeMessageSender can send node messages.
type NodeMessageSender interface {
	// SendMessage submits a message to the node pubsub.
	SendMessage(ctx context.Context, msgType inca.NodeMessageType, msgInnerRef *storageref.StorageRef) error
}
