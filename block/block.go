package block

import (
	"context"
	"encoding/base64"

	"github.com/aperturerobotics/hydra/block"
	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/hydra/object"
	"github.com/aperturerobotics/inca"
	iblock "github.com/aperturerobotics/inca/block"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

// BlockCommitRatio is the ratio of voting power committed to not committed to finalize a block.
const BlockCommitRatio float32 = 0.66

// Block is a block header wrapped with some context.
type Block struct {
	iblock.State
	//	dbm      db.Db
	blk          *inca.Block
	header       *inca.BlockHeader
	headerCursor *block.Cursor
	blkRef       *cid.BlockRef
	id           string
}

// GetBlock gets the Block object associated with the given Block storage ref.
func GetBlock(
	ctx context.Context,
	cursor *block.Cursor,
	store object.ObjectStore,
) (*Block, error) {
	blockRefKey, err := cursor.GetRef().MarshalKey()
	if err != nil {
		return nil, err
	}
	blki, err := cursor.Unmarshal(func() block.Block { return &inca.Block{} })
	if err != nil {
		return nil, err
	}
	blk := blki.(*inca.Block)

	blkHeaderCursor, err := cursor.FollowRef(1, blk.GetBlockHeaderRef())
	if err != nil {
		return nil, err
	}
	blkHeaderi, err := blkHeaderCursor.Unmarshal(func() block.Block {
		return &inca.BlockHeader{}
	})
	blkHeader := blkHeaderi.(*inca.BlockHeader)

	id := base64.StdEncoding.EncodeToString(blockRefKey)
	b := &Block{
		id: id,
		// dbm:      dbm,
		blk:          blk,
		header:       blkHeader,
		blkRef:       cursor.GetRef(),
		headerCursor: blkHeaderCursor,
	}

	if err := b.ReadState(store); err != nil {
		return nil, err
	}

	return b, nil
}

// GetDbKey returns the key of this block.
func (b *Block) GetDbKey() string {
	return b.id
}

// ReadState loads the peer state from the database.
func (b *Block) ReadState(store object.ObjectStore) error {
	dbKey := b.GetDbKey()
	dat, datOk, err := store.GetObject(dbKey)
	if err != nil || !datOk {
		return err
	}
	return proto.Unmarshal(dat, &b.State)
}

// WriteState writes the last observed state and other parameters to the db.
func (b *Block) WriteState(store object.ObjectStore) error {
	dbKey := b.GetDbKey()
	dat, err := proto.Marshal(&b.State)
	if err != nil {
		return err
	}

	return store.SetObject(dbKey, dat)
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

// ValidateChild checks if a block can be the next in the sequence.
// TODO: validate timestamps on round
// Returns markValid, which indicates that the block should be considered valid without a known parent.
func (b *Block) ValidateChild(
	ctx context.Context,
	child *Block,
	blockValidator Validator,
) (bool, error) {
	bHeader := b.header
	childHeader := child.header

	if !bHeader.GetGenesisRef().EqualsRef(childHeader.GetGenesisRef()) {
		return false, errors.New("genesis reference mismatch")
	}

	bTs := b.header.GetBlockTs().ToTime()
	childTs := childHeader.GetBlockTs().ToTime()

	bRoundInfo := bHeader.GetRoundInfo()
	childRoundInfo := childHeader.GetRoundInfo()
	// TODO: validate round info is not empty (maybe?)

	childExpectedHeight := bRoundInfo.GetHeight() + 1
	if childRoundInfo.GetHeight() != childExpectedHeight {
		return false, errors.Errorf("child height %d != expected %d", childRoundInfo.GetHeight(), childExpectedHeight)
	}

	if childTs.Before(bTs) {
		return false, errors.Errorf("child timestamp %s before parent %s", childTs.String(), bTs.String())
	}

	chainConfCursor, err := b.headerCursor.FollowRef(2, b.GetHeader().GetChainConfigRef())
	if err != nil {
		return false, err
	}
	chainConfi, err := chainConfCursor.Unmarshal(func() block.Block { return &inca.ChainConfig{} })
	if err != nil {
		return false, err
	}
	chainConf := chainConfi.(*inca.ChainConfig)

	// TODO: allow mutataing chain config
	if !bHeader.GetChainConfigRef().EqualsRef(childHeader.GetChainConfigRef()) {
		return false, errors.New("child chain config does not match parent")
	}

	if !bHeader.GetChainConfigRef().EqualsRef(childHeader.GetNextChainConfigRef()) {
		return false, errors.New("child next chain config does not match parent")
	}

	valSetCursor, err := chainConfCursor.FollowRef(2, chainConf.GetValidatorSetRef())
	if err != nil {
		return false, err
	}
	valSeti, err := valSetCursor.Unmarshal(func() block.Block { return &inca.ValidatorSet{} })
	if err != nil {
		return false, err
	}
	valSet := valSeti.(*inca.ValidatorSet)

	brKey, err := b.blkRef.MarshalKey()
	if err != nil {
		return false, err
	}
	selValidator, _ := valSet.SelectProposer(
		brKey,
		childRoundInfo.GetHeight(),
		childRoundInfo.GetRound(),
	)
	if selValidator == nil {
		return false, errors.New("selected validator was nil")
	}

	valSetValidators := valSet.GetValidators()
	childValidatorIdx := int(childHeader.GetValidatorIndex())
	if len(valSetValidators) >= childValidatorIdx {
		return false, errors.Errorf(
			"previous validator index out of range: %d >= %d",
			childValidatorIdx,
			len(valSetValidators),
		)
	}
	childValidator := valSetValidators[childValidatorIdx]
	childValidatorID, childValidatorPub, err := childValidator.ParsePeerID()
	if err != nil {
		return false, errors.Wrap(err, "child validator public key invalid")
	}

	selValidatorID, _, err := selValidator.ParsePeerID()
	if err != nil {
		return false, err
	}

	if !selValidatorID.MatchesPublicKey(childValidatorPub) {
		return false, errors.Errorf(
			"expected validator for (%d, %d) %s != actual %s",
			childRoundInfo.GetHeight(),
			childRoundInfo.GetRound(),
			selValidatorID.Pretty(),
			childValidatorID.Pretty(),
		)
	}

	// TODO: decide if this is always required.
	if blockValidator != nil {
		return blockValidator.ValidateBlock(ctx, child, b)
	}

	return false, nil
}

// GetBlockRef returns the block ref.
func (b *Block) GetBlockRef() *cid.BlockRef {
	return b.blkRef
}

// GetStateRef returns the state ref (shortcut).
func (b *Block) GetStateRef() *cid.BlockRef {
	return b.GetHeader().GetStateRef()
}
