package block

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/golang/protobuf/proto"
)

// BlockCommitRatio is the ratio of voting power committed to not committed to finalize a block.
const BlockCommitRatio float32 = 0.66

// Block is a block header wrapped with some context.
type Block struct {
	state    State
	dbm      db.Db
	blk      *inca.Block
	header   *inca.BlockHeader
	encStrat encryption.Strategy
	blkRef   *storageref.StorageRef
	dbKey    []byte
}

// FollowBlockRef follows a reference to a Block object.
func FollowBlockRef(ctx context.Context, ref *storageref.StorageRef, encStrat encryption.Strategy) (*inca.Block, error) {
	blk := &inca.Block{}
	encConf := encStrat.GetBlockEncryptionConfigWithDigest(ref.GetObjectDigest())
	subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
	if err := ref.FollowRef(subCtx, ref.GetObjectDigest(), blk); err != nil {
		return nil, err
	}
	return blk, nil
}

// FollowBlockHeaderRef follows a reference to a BlockHeader object.
func FollowBlockHeaderRef(ctx context.Context, ref *storageref.StorageRef, encStrat encryption.Strategy) (*inca.BlockHeader, error) {
	blk := &inca.BlockHeader{}
	encConf := encStrat.GetBlockEncryptionConfigWithDigest(ref.GetObjectDigest())
	subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
	if err := ref.FollowRef(subCtx, ref.GetObjectDigest(), blk); err != nil {
		return nil, err
	}
	return blk, nil
}

// GetBlock gets the Block object associated with the given Block storage ref.
func GetBlock(
	ctx context.Context,
	encStrat encryption.Strategy,
	dbm db.Db,
	blockRef *storageref.StorageRef,
) (*Block, error) {
	blk, err := FollowBlockRef(ctx, blockRef, encStrat)
	if err != nil {
		return nil, err
	}

	blkHeader, err := FollowBlockHeaderRef(ctx, blk.GetBlockHeaderRef(), encStrat)
	if err != nil {
		return nil, err
	}

	b := &Block{
		dbm:      dbm,
		blk:      blk,
		encStrat: encStrat,
		header:   blkHeader,
		blkRef:   blockRef,
	}
	b.dbKey = b.computeDbKey()

	if err := b.ReadState(ctx); err != nil {
		return nil, err
	}

	return b, nil
}

// computeDbKey computes the db key.
func (b *Block) computeDbKey() []byte {
	return []byte(fmt.Sprintf("/%s", base64.StdEncoding.EncodeToString(b.blkRef.GetObjectDigest())))
}

// GetDbKey returns the key of this block.
func (b *Block) GetDbKey() []byte {
	return b.dbKey
}

// ReadState loads the peer state from the database.
func (b *Block) ReadState(ctx context.Context) error {
	dbKey := b.GetDbKey()
	dat, err := b.dbm.Get(ctx, dbKey)
	if err != nil {
		return err
	}
	if dat == nil {
		return nil
	}
	return proto.Unmarshal(dat, &b.state)
}

// WriteState writes the last observed state and other parameters to the db.
func (b *Block) WriteState(ctx context.Context) error {
	dbKey := b.GetDbKey()
	dat, err := proto.Marshal(&b.state)
	if err != nil {
		return err
	}

	return b.dbm.Set(ctx, dbKey, dat)
}

// GetHeader returns the header.
func (b *Block) GetHeader() *inca.BlockHeader {
	return b.header
}

// GetInnerBlock returns the inner block object.
func (b *Block) GetInnerBlock() *inca.Block {
	return b.blk
}
