package txdb

import (
	"context"

	"github.com/golang/protobuf/proto"
)

// getIDKey returns the key for the given ID.
func (h *TxDatabase) getIDKey(id string) []byte {
	return append([]byte{'/'}, []byte(id)...)
}

// getEntry gets the entry with the specified ID from the db.
func (h *TxDatabase) getEntry(
	ctx context.Context,
	id string,
) (*TransactionInfo, error) {
	if id == "" {
		return nil, nil
	}

	idKey := h.getIDKey(id)
	d, dOk, err := h.db.Get(ctx, idKey)
	if err != nil {
		return nil, err
	}

	if !dOk {
		return nil, nil
	}

	entry := &TransactionInfo{}
	if err := proto.Unmarshal(d, entry); err != nil {
		return nil, err
	}

	return entry, nil
}

// setEntry sets the entry with the specified ID to the db.
func (h *TxDatabase) setEntry(ctx context.Context, id string, entry *TransactionInfo) error {
	idKey := h.getIDKey(id)

	dat, err := proto.Marshal(entry)
	if err != nil {
		return err
	}

	return h.db.Set(ctx, idKey, dat)
}
