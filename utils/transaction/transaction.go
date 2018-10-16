package transaction

import (
	"crypto/sha1"
	"encoding/binary"
	"encoding/hex"
	"strings"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"
	"github.com/libp2p/go-libp2p-crypto"
	lcrypto "github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
)

// Transaction contains data about a transaction.
type Transaction struct {
	id string

	nodeMessage    *inca.NodeMessage
	nodeMessageRef *storageref.StorageRef
	innerRef       *storageref.StorageRef
}

// GetObjectTypeID returns the object type string, used to identify types.
func (t *Transaction) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/util/transaction")
}

// GetObjectTypeID returns the object type string, used to identify types.
func (t *TransactionSet) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/util/transaction/set")
}

// GetObjectTypeID returns the object type string, used to identify types.
func (s *BlockState) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/util/transaction/block-state")
}

// TransactionAppMessageID is the identifier for the app message for a transaction.
var TransactionAppMessageID = (&Transaction{}).GetObjectTypeID().GetCrc32()

// GetID returns the transaction ID.
func (t *Transaction) GetID() string {
	return t.id
}

// GetNodeMessage returns the NodeMessage used to build this transaction if any.
func (t *Transaction) GetNodeMessage() *inca.NodeMessage {
	return t.nodeMessage
}

// GetInnerRef returns the inner reference.
func (t *Transaction) GetInnerRef() *storageref.StorageRef {
	return t.nodeMessage.GetInnerRef()
}

// GetNodeMessageRef returns the node message ref if set by SetNodeMessageRef.
func (t *Transaction) GetNodeMessageRef() *storageref.StorageRef {
	return t.nodeMessageRef
}

// SetNodeMessageRef sets the storage ref.
func (t *Transaction) SetNodeMessageRef(nodeMessageRef *storageref.StorageRef) {
	t.nodeMessageRef = nodeMessageRef
}

// FromNodeMessage loads a transaction from a node message.
func FromNodeMessage(
	nodeMessage *inca.NodeMessage,
) (*Transaction, error) {
	nodeKey, err := lcrypto.UnmarshalPublicKey(nodeMessage.GetPubKey())
	if err != nil {
		return nil, err
	}

	if nodeMessage.GetMessageType() != inca.NodeMessageType_NodeMessageType_APP {
		return nil, errors.New("expected app node message type for transaction")
	}

	if nodeMessage.GetAppMessageType() != TransactionAppMessageID {
		return nil, errors.Errorf(
			"expected transaction app message id %d for tx, given: %d",
			TransactionAppMessageID,
			nodeMessage.GetAppMessageType(),
		)
	}

	timestamp := nodeMessage.GetTimestamp()
	timestampUnix := timestamp.GetTimeUnixMs()
	if timestampUnix == 0 {
		return nil, errors.New("expected timestamped nodemessage for transaction")
	}

	tid, err := ComputeTxID(nodeMessage.GetTimestamp(), nodeKey)
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
func ComputeTxID(messageTimestamp *timestamp.Timestamp, nodeKey crypto.PubKey) (string, error) {
	h := sha1.New()
	unixMs := messageTimestamp.GetTimeUnixMs()
	if err := binary.Write(h, binary.LittleEndian, unixMs); err != nil {
		return "", err
	}

	keyBytes, err := nodeKey.Bytes()
	if err != nil {
		return "", nil
	}

	_, _ = h.Write(keyBytes)
	shaSum := h.Sum(nil)
	return strings.ToLower(hex.EncodeToString(shaSum)), nil
}
