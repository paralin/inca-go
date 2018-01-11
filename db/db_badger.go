package db

import (
	"context"

	"github.com/aperturerobotics/inca-go/objtable"
	"github.com/aperturerobotics/pbobject"
	"github.com/dgraph-io/badger"
	"github.com/golang/protobuf/proto"
)

// BadgerDB implements Db with badger.
type BadgerDB struct {
	*badger.DB
	table *objtable.ObjectTable
}

// NewBadgerDB builds a new badger database.
func NewBadgerDB(db *badger.DB, objectTable *objtable.ObjectTable) Db {
	return &BadgerDB{DB: db, table: objectTable}
}

// Get retrieves an object from the database.
// Not found should return nil, nil
func (d *BadgerDB) Get(ctx context.Context, key []byte) (pbobject.Object, error) {
	var objWrapper *pbobject.ObjectWrapper
	getErr := d.View(func(txn *badger.Txn) error {
		item, rerr := txn.Get(key)
		if rerr != nil {
			if rerr == badger.ErrKeyNotFound {
				return nil
			}
			return rerr
		}

		val, err := item.Value()
		if err != nil {
			return err
		}

		objWrapper := &pbobject.ObjectWrapper{}
		return proto.Unmarshal(val, objWrapper)
	})
	if getErr != nil {
		return nil, getErr
	}
	if objWrapper == nil {
		return nil, nil
	}

	return d.table.DecodeWrapper(ctx, objWrapper)
}

// Set sets an object in the database.
func (d *BadgerDB) Set(ctx context.Context, key []byte, val pbobject.Object) error {
	ow, err := d.table.Encode(ctx, val)
	if err != nil {
		return err
	}

	dat, err := proto.Marshal(ow)
	if err != nil {
		return err
	}

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, dat)
	})
}

// List lists keys in the database.
func (d *BadgerDB) List(ctx context.Context, prefix []byte) ([][]byte, error) {
	var vals [][]byte
	err := d.DB.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			item := it.Item()
			k := item.Key()
			kb := make([]byte, len(k))
			copy(kb, k)
			vals = append(vals, kb)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return vals, nil
}
