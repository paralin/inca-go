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
	"github.com/pkg/errors"
)

// Transaction contains data about a transaction.
type Transaction struct {
	id string

	nodeMessage *inca.NodeMessage
	storageRef  *storageref.StorageRef
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

// GetStorageRef returns the storage ref if set by SetStorageRef.
func (t *Transaction) GetStorageRef() *storageref.StorageRef {
	return t.storageRef
}

// SetStorageRef sets the storage ref.
func (t *Transaction) SetStorageRef(storageRef *storageref.StorageRef) {
	t.storageRef = storageRef
}

// FromNodeMessage loads a transaction from a node message object wrapper and message.
// It is expected that nodeMessage was decoded from obj.
// The signature of the message and other data is validated.
func FromNodeMessage(
	obj *pbobject.ObjectWrapper,
	nodeMessage *inca.NodeMessage,
	nodeKey crypto.PubKey,
) (*Transaction, error) {
	if len(obj.GetSignatures()) != 1 {
		return nil, errors.New("expected a single signature on a transaction message wrapper")
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

	if nodeKey != nil {
		sig := obj.GetSignatures()[0]
		if err := sig.MatchesPublicKey(nodeKey); err != nil {
			return nil, err
		}
	}

	tid, err := ComputeTxID(nodeMessage.GetTimestamp(), nodeKey)
	if err != nil {
		return nil, err
	}

	return &Transaction{id: tid, nodeMessage: nodeMessage}, nil
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
