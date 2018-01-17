package localdb

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/golang/protobuf/proto"
	mh "github.com/multiformats/go-multihash"
)

var localDbHashCode uint64 = mh.SHA2_256
var localDbHashLen = -1

// LocalDb wraps a db.Db to implement LocalStore.
type LocalDb struct {
	db.Db
}

// NewLocalDb builds a new LocalDb.
func NewLocalDb(db db.Db) *LocalDb {
	return &LocalDb{Db: db}
}

// GetDigestKey returns the key for the given digest.
func (l *LocalDb) GetDigestKey(hash []byte) []byte {
	hashHex := hex.EncodeToString(hash)
	return []byte(fmt.Sprintf("/%s", hashHex))
}

// DigestData digests the data.
func (l *LocalDb) DigestData(data []byte) ([]byte, error) {
	m, err := mh.Sum(data, localDbHashCode, localDbHashLen)
	if err != nil {
		return nil, err
	}

	dmh, err := mh.Decode(m)
	if err != nil {
		return nil, err
	}

	return dmh.Digest, nil
}

// GetLocal returns an object by digest, assuming it has already been fetched into the decrypted cache.
// The hash is of the innermost data of the object, unencrypted, without the multihash header.
// If not found, returns not found error.
func (l *LocalDb) GetLocal(ctx context.Context, digest []byte, obj pbobject.Object) error {
	dat, err := l.Db.Get(ctx, l.GetDigestKey(digest))
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return objstore.ErrNotFound
	}

	return proto.Unmarshal(dat, obj)
}

// StoreLocal encodes an object to an unencrypted blob, hashing it with the database hashing scheme.
// hashPtr is a pointer to the expected unencrypted hash of the data. If the target array is nil,
// the target will be written with the computed hash and not verified before storing.
// If the target array is not nil, the hash will be checked before storage.
func (l *LocalDb) StoreLocal(ctx context.Context, object pbobject.Object, hashPtr *[]byte, params objstore.StoreParams) error {
	var digest []byte
	if hashPtr != nil {
		digest = *hashPtr
	}

	val, err := proto.Marshal(object)
	if err != nil {
		return err
	}

	computedDigest, err := l.DigestData(val)
	if err != nil {
		return err
	}

	if len(digest) != 0 {
		if bytes.Compare(digest, computedDigest) != 0 {
			return errors.New("digest of encoded data did not match given digest")
		}
	} else if hashPtr != nil {
		*hashPtr = computedDigest
	}

	digest = computedDigest
	return l.Db.Set(ctx, l.GetDigestKey(digest), val)
}

// _ is a type assertion
var _ objstore.LocalStore = &LocalDb{}
