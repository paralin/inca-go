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
func (d *BadgerDB) Get(ctx context.Context, key string) (pbobject.Object, error) {
	var objWrapper *pbobject.ObjectWrapper
	getErr := d.View(func(txn *badger.Txn) error {
		item, rerr := txn.Get([]byte(key))
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
func (d *BadgerDB) Set(ctx context.Context, key string, val pbobject.Object) error {
	ow, err := d.table.Encode(ctx, val)
	if err != nil {
		return err
	}

	dat, err := proto.Marshal(ow)
	if err != nil {
		return err
	}

	return d.DB.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(key), dat)
	})
}
