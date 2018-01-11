package db

import (
	"bytes"
	"context"
	"crypto/sha1"
	"sync"

	"github.com/aperturerobotics/pbobject"
)

// inMemDb is a in-memory database.
type inMemDb struct {
	m  sync.Map // map[[sha1.Size]byte]pbobject.Object
	mk sync.Map // map[[sha1.Size]byte][]byte
}

// NewInmemDb returns a in-memory database.
func NewInmemDb() Db {
	return &inMemDb{}
}

// hashKey hashes a key.
func (m *inMemDb) hashKey(key []byte) [sha1.Size]byte {
	return sha1.Sum(key)
}

// Get retrieves an object from the database.
// Not found should return nil, nil
func (m *inMemDb) Get(ctx context.Context, key []byte) (pbobject.Object, error) {
	k := m.hashKey(key)
	obj, ok := m.m.Load(k)
	if !ok {
		return nil, nil
	}

	return obj.(pbobject.Object), nil
}

// Set sets an object in the database.
func (m *inMemDb) Set(ctx context.Context, key []byte, val pbobject.Object) error {
	k := m.hashKey(key)
	m.m.Store(k, val)
	m.mk.Store(k, key)
	return nil
}

// List returns a list of keys with the specified prefix.
func (m *inMemDb) List(ctx context.Context, prefix []byte) ([][]byte, error) {
	var ks [][]byte
	m.m.Range(func(key interface{}, value interface{}) bool {
		keyHash := key.([sha1.Size]byte)
		key, ok := m.mk.Load(keyHash)
		if !ok {
			return true
		}

		kb := key.([]byte)
		if bytes.HasPrefix(kb, prefix) {
			ks = append(ks, kb)
		}
		return true
	})

	return ks, nil
}
