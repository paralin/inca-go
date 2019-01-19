package transaction

import (
	"bytes"
	"encoding/binary"

	"github.com/aperturerobotics/bifrost/hash"
	lpeer "github.com/aperturerobotics/bifrost/peer"
	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/timestamp"
	"github.com/pkg/errors"
)

// Transaction contains data about a transaction.
type Transaction struct {
	id string

	nodeMessage    *inca.NodeMessage
	nodeMessageRef *cid.BlockRef
	innerRef       *cid.BlockRef
}

// GetID returns the transaction ID.
func (t *Transaction) GetID() string {
	return t.id
}

// GetNodeMessage returns the NodeMessage used to build this transaction if any.
func (t *Transaction) GetNodeMessage() *inca.NodeMessage {
	return t.nodeMessage
}

// GetInnerRef returns the inner reference.
func (t *Transaction) GetInnerRef() *cid.BlockRef {
	return t.nodeMessage.GetInnerRef()
}

// GetNodeMessageRef returns the node message ref if set by SetNodeMessageRef.
func (t *Transaction) GetNodeMessageRef() *cid.BlockRef {
	return t.nodeMessageRef
}

// SetNodeMessageRef sets the storage ref.
func (t *Transaction) SetNodeMessageRef(nodeMessageRef *cid.BlockRef) {
	t.nodeMessageRef = nodeMessageRef
}

// FromNodeMessage loads a transaction from a node message.
func FromNodeMessage(
	peerID lpeer.ID,
	nodeMessage *inca.NodeMessage,
) (*Transaction, error) {
	if nodeMessage.GetMessageType() != inca.NodeMessageType_NodeMessageType_APP {
		return nil, errors.New("expected app node message type for transaction")
	}

	/* TODO
	if nodeMessage.GetAppMessageType() != TransactionAppMessageID {
		return nil, errors.Errorf(
			"expected transaction app message id %d for tx, given: %d",
			TransactionAppMessageID,
			nodeMessage.GetAppMessageType(),
		)
	}
	*/

	timestamp := nodeMessage.GetTimestamp()
	timestampUnix := timestamp.GetTimeUnixMs()
	if timestampUnix == 0 {
		return nil, errors.New("expected timestamped nodemessage for transaction")
	}

	tid, err := ComputeTxID(nodeMessage.GetTimestamp(), peerID)
	if err != nil {
		return nil, err
	}

	return &Transaction{
		id:          tid,
		nodeMessage: nodeMessage,
		innerRef:    nodeMessage.GetInnerRef(),
	}, nil
}

// ComputeTxID computes a transaction ID from a timestamp and node key.
func ComputeTxID(messageTimestamp *timestamp.Timestamp, nodePeerID lpeer.ID) (string, error) {
	var buf bytes.Buffer
	unixMs := messageTimestamp.GetTimeUnixMs()
	if err := binary.Write(&buf, binary.LittleEndian, unixMs); err != nil {
		return "", err
	}
	keyBytes := []byte(nodePeerID.Pretty())
	if _, err := (&buf).Write(keyBytes); err != nil {
		return "", err
	}

	h, err := hash.Sum(hash.HashType_HashType_SHA256, (&buf).Bytes())
	if err != nil {
		return "", err
	}
	return h.MarshalString(), nil
}
