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
	"github.com/pkg/errors"
)

// BlockCommitRatio is the ratio of voting power committed to not committed to finalize a block.
const BlockCommitRatio float32 = 0.66

// Block is a block header wrapped with some context.
type Block struct {
	State
	dbm      db.Db
	blk      *inca.Block
	header   *inca.BlockHeader
	encStrat encryption.Strategy
	blkRef   *storageref.StorageRef
	id       string
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
		id:       base64.StdEncoding.EncodeToString(blockRef.GetObjectDigest()),
		dbm:      dbm,
		blk:      blk,
		encStrat: encStrat,
		header:   blkHeader,
		blkRef:   blockRef,
	}
	b.dbKey = []byte(fmt.Sprintf("/%s", b.id))

	if err := b.ReadState(ctx); err != nil {
		return nil, err
	}

	return b, nil
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
	return proto.Unmarshal(dat, &b.State)
}

// WriteState writes the last observed state and other parameters to the db.
func (b *Block) WriteState(ctx context.Context) error {
	dbKey := b.GetDbKey()
	dat, err := proto.Marshal(&b.State)
	if err != nil {
		return err
	}

	return b.dbm.Set(ctx, dbKey, dat)
}

// GetId returns the block Id
func (b *Block) GetId() string {
	return b.id
}

// GetHeader returns the header.
func (b *Block) GetHeader() *inca.BlockHeader {
	return b.header
}

// GetInnerBlock returns the inner block object.
func (b *Block) GetInnerBlock() *inca.Block {
	return b.blk
}

func (b *Block) fetchChainConfig(ctx context.Context) (*inca.ChainConfig, error) {
	chainConf := &inca.ChainConfig{}
	chainConfRef := b.header.GetChainConfigRef()
	encConf := b.encStrat.GetBlockEncryptionConfigWithDigest(chainConfRef.GetObjectDigest())
	subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
	if err := chainConfRef.FollowRef(subCtx, nil, chainConf); err != nil {
		return nil, err
	}

	return chainConf, nil
}

// ValidateChild checks if a block can be the next in the sequence.
// TODO: validate timestamps on round
func (b *Block) ValidateChild(ctx context.Context, child *Block) error {
	bHeader := b.header
	childHeader := child.header

	bTs := b.header.GetBlockTs().ToTime()
	childTs := childHeader.GetBlockTs().ToTime()

	bRoundInfo := bHeader.GetRoundInfo()
	childRoundInfo := childHeader.GetRoundInfo()

	childExpectedHeight := bRoundInfo.GetHeight() + 1
	if childRoundInfo.GetHeight() != childExpectedHeight {
		return errors.Errorf("child height %d != expected %d", childRoundInfo.GetHeight(), childExpectedHeight)
	}

	if childTs.Before(bTs) {
		return errors.Errorf("child timestamp %s before parent %s", childTs.String(), bTs.String())
	}

	chainConf, err := b.fetchChainConfig(ctx)
	if err != nil {
		return err
	}

	// TODO: allow mutataing chain config
	if !bHeader.GetChainConfigRef().Equals(childHeader.GetChainConfigRef()) {
		return errors.New("child chain config does not match parent")
	}

	if !bHeader.GetChainConfigRef().Equals(childHeader.GetNextChainConfigRef()) {
		return errors.New("child next chain config does not match parent")
	}

	valSet := &inca.ValidatorSet{}
	{
		encConf := b.encStrat.GetBlockEncryptionConfigWithDigest(chainConf.GetValidatorSetRef().GetObjectDigest())
		subCtx := pbobject.WithEncryptionConf(ctx, &encConf)
		if err := chainConf.GetValidatorSetRef().FollowRef(subCtx, nil, valSet); err != nil {
			return err
		}
	}

	selValidator, _ := valSet.SelectProposer(
		b.blkRef.GetObjectDigest(),
		childRoundInfo.GetHeight(),
		childRoundInfo.GetRound(),
	)
	if selValidator == nil {
		return errors.New("selected validator was nil")
	}

	childValidatorID := childHeader.GetProposerId()
	selValidatorID, _, err := selValidator.ParsePeerID()
	if err != nil {
		return err
	}

	if selValidatorID.Pretty() != childValidatorID {
		return errors.Errorf("expected validator for (%d, %d) %s != actual %s", childRoundInfo.GetHeight(), childRoundInfo.GetRound(), selValidatorID.Pretty(), childValidatorID)
	}

	return nil
}
