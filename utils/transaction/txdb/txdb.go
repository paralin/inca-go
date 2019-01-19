package txdb

import (
	"context"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/golang/protobuf/proto"
)

// TxDatabase stores all known transactions with their states.
type TxDatabase struct {
	// db is the underlying datastore
	db db.Db
}

// NewTxDatabase constructs a new transaction database, loading state from the db.
func NewTxDatabase(
	ctx context.Context,
	dbm db.Db,
) (*TxDatabase, error) {
	return &TxDatabase{db: dbm}, nil
}

// Get returns the transaction by ID.
func (d *TxDatabase) Get(ctx context.Context, txID string) (*TransactionInfo, error) {
	return d.getEntry(ctx, txID)
}

// GetTx returns the transaction info and resolves the transaction object.
func (d *TxDatabase) GetTx(ctx context.Context, txID string) (*transaction.Transaction, error) {
	info, err := d.getEntry(ctx, txID)
	if err != nil || info == nil {
		return nil, err
	}

	nm := &inca.NodeMessage{}
	nmRef := info.GetNodeMessageRef()
	if err := nmRef.FollowRef(ctx, nil, nm, nil); err != nil {
		return nil, err
	}

	tx, err := transaction.FromNodeMessage(nm)
	if err != nil {
		return nil, err
	}

	tx.SetNodeMessageRef(nmRef)
	return tx, nil
}

// Put puts a new transaction into the db.
func (d *TxDatabase) Put(ctx context.Context, txID string, nodeMessageRef *storageref.StorageRef) error {
	key := d.getIDKey(txID)
	info := &TransactionInfo{
		NodeMessageRef: nodeMessageRef,
	}

	dat, err := proto.Marshal(info)
	if err != nil {
		return err
	}

	return d.db.Set(ctx, key, dat)
}
