package db

import (
	"context"

	"github.com/dgraph-io/badger"
)

// BadgerDB implements Db with badger.
type BadgerDB struct {
	*badger.DB
}

// NewBadgerDB builds a new badger database.
func NewBadgerDB(db *badger.DB) Db {
	return &BadgerDB{DB: db}
}

// Get retrieves an object from the database.
// Not found should return nil, nil
func (d *BadgerDB) Get(ctx context.Context, key []byte) ([]byte, error) {
	var objVal []byte
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

		objVal = val
		return nil
	})
	return objVal, getErr
}

// Set sets an object in the database.
func (d *BadgerDB) Set(ctx context.Context, key []byte, val []byte) error {
	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set(key, val)
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
