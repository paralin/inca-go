package chain

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
	"github.com/aperturerobotics/inca-go/segment"
	ichain "github.com/aperturerobotics/inca/chain"
	isegment "github.com/aperturerobotics/inca/segment"

	"github.com/aperturerobotics/objstore"
	idb "github.com/aperturerobotics/objstore/db"

	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"

	"github.com/libp2p/go-libp2p-crypto"
	lpeer "github.com/libp2p/go-libp2p-peer"

	// _ enables the IPFS storage ref type
	_ "github.com/aperturerobotics/storageref/ipfs"
)

var genesisKey = "/genesis"

// Chain is an instance of a blockchain.
type Chain struct {
	ctx                 context.Context
	db                  *objstore.ObjectStore
	dbm                 idb.Db
	conf                *ichain.Config
	genesis             *inca.Genesis
	encStrat            encryption.Strategy
	le                  *logrus.Entry
	state               ichain.ChainState
	recheckStateTrigger chan struct{}
	pubsubTopic         string

	stateSnapshotCtx       context.Context
	stateSnapshotCtxCancel context.CancelFunc
	lastStateSnapshot      *state.ChainStateSnapshot

	stateSubs       sync.Map
	lastHeadDigest  []byte
	lastBlock       *inca.Block
	lastBlockHeader *inca.BlockHeader
	lastBlockRef    *storageref.StorageRef
	segStore        *segment.SegmentStore

	blockValidator block.Validator
	blockProposer  block.Proposer
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	dbm idb.Db,
	db *objstore.ObjectStore,
	chainID string,
	validatorPriv crypto.PrivKey,
	blockValidator block.Validator,
) (*Chain, error) {
	if chainID == "" {
		return nil, errors.New("chain id must be set")
	}

	le := logctx.GetLogEntry(ctx)
	if objstore.GetObjStore(ctx) == nil {
		ctx = objstore.WithObjStore(ctx, db)
	}

	strat, err := convergentimmutable.NewConvergentImmutableStrategy()
	if err != nil {
		return nil, err
	}

	argsObjectWrapper, _, err := strat.BuildArgs()
	if err != nil {
		return nil, err
	}

	validatorPub := validatorPriv.GetPublic()
	validatorPubBytes, err := validatorPub.Bytes()
	if err != nil {
		return nil, err
	}

	validatorID, err := lpeer.IDFromPublicKey(validatorPub)
	if err != nil {
		return nil, err
	}

	validatorSet := &inca.ValidatorSet{
		Validators: []*inca.Validator{
			&inca.Validator{
				PubKey:        validatorPubBytes,
				VotingPower:   10,
				OperationMode: inca.Validator_OperationMode_OPERATING,
			},
		},
	}
	validatorSetStorageRef, _, err := db.StoreObject(
		ctx,
		validatorSet,
		strat.GetBlockEncryptionConfig(),
	)
	if err != nil {
		return nil, err
	}

	chainConf := &inca.ChainConfig{
		TimingConfig: &inca.TimingConfig{
			MinProposeAfterBlock: 500,
			RoundLength:          3000,
		},
		ValidatorSetRef: validatorSetStorageRef,
	}
	chainConfStorageRef, _, err := db.StoreObject(
		ctx,
		chainConf,
		strat.GetBlockEncryptionConfig(),
	)
	if err != nil {
		return nil, err
	}

	genesisTs := timestamp.Now()
	genesis := &inca.Genesis{
		ChainId:            chainID,
		Timestamp:          &genesisTs,
		EncStrategy:        strat.GetEncryptionStrategyType(),
		InitChainConfigRef: chainConfStorageRef,
	}

	genesisStorageRef, _, err := db.StoreObject(
		ctx,
		genesis,
		strat.GetGenesisEncryptionConfig(),
	)
	if err != nil {
		return nil, err
	}

	conf := &ichain.Config{
		GenesisRef:         genesisStorageRef,
		EncryptionStrategy: strat.GetEncryptionStrategyType(),
	}

	ch := &Chain{
		ctx:     ctx,
		conf:    conf,
		genesis: genesis,
		db:      db,
		dbm:     dbm,
		le:      le,

		blockValidator:      blockValidator,
		recheckStateTrigger: make(chan struct{}, 1),
	}
	ch.segStore = segment.NewSegmentStore(
		ctx,
		idb.WithPrefix(
			dbm,
			[]byte(fmt.Sprintf("/chain/%s/segments", ch.genesis.GetChainId()))),
		db,
		ch.GetEncryptionStrategy(),
		ch.GetBlockValidator(),
		ch.GetBlockDbm(),
	)

	conf.EncryptionArgs = argsObjectWrapper
	ch.encStrat = strat
	ch.computePubsubTopic()

	// build the first block
	nowTs := timestamp.Now()
	firstBlockHeaderEncConf := strat.GetBlockEncryptionConfig()
	firstBlockHeaderEncConf.SignerKeys = []crypto.PrivKey{validatorPriv}
	firstBlockHeader := &inca.BlockHeader{
		GenesisRef:         genesisStorageRef,
		ChainConfigRef:     chainConfStorageRef,
		NextChainConfigRef: chainConfStorageRef,
		RoundInfo: &inca.BlockRoundInfo{
			Height: 0,
			Round:  0,
		},
		BlockTs:    &nowTs,
		ProposerId: validatorID.Pretty(),
	}

	firstBlockHeaderStorageRef, _, err := db.StoreObject(
		ctx,
		firstBlockHeader,
		firstBlockHeaderEncConf,
	)
	if err != nil {
		return nil, err
	}

	// build a Vote for the first block
	firstBlockVoteEncConf := strat.GetNodeMessageEncryptionConfig(validatorPriv)
	firstBlockVote := &inca.Vote{
		BlockHeaderRef: firstBlockHeaderStorageRef,
	}
	firstBlockVoteStorageRef, _, err := db.StoreObject(
		ctx,
		firstBlockVote,
		firstBlockVoteEncConf,
	)
	if err != nil {
		return nil, err
	}

	firstBlock := &inca.Block{
		BlockHeaderRef: firstBlockHeaderStorageRef,
		VoteRefs: []*storageref.StorageRef{
			firstBlockVoteStorageRef,
		},
	}
	firstBlockStorageRef, _, err := db.StoreObject(
		ctx,
		firstBlock,
		firstBlockHeaderEncConf,
	)
	if err != nil {
		return nil, err
	}

	var firstSegDigest []byte
	uid, _ := uuid.NewV4()
	firstSeg := &isegment.SegmentState{
		Id:        uid.String(),
		Status:    isegment.SegmentStatus_SegmentStatus_VALID,
		HeadBlock: firstBlockStorageRef,
		TailBlock: firstBlockStorageRef,
	}
	if err := db.LocalStore.StoreLocal(
		ctx,
		firstSeg,
		&firstSegDigest,
		objstore.StoreParams{},
	); err != nil {
		return nil, err
	}

	firstBlk, err := block.GetBlock(
		ctx,
		ch.encStrat,
		blockValidator,
		ch.GetBlockDbm(),
		firstBlockStorageRef,
	)
	if err != nil {
		return nil, err
	}

	seg, err := ch.segStore.NewSegment(ctx, firstBlk, firstBlockStorageRef)
	if err != nil {
		return nil, err
	}

	firstBlk.State.SegmentId = seg.GetId()
	if err := firstBlk.WriteState(ctx); err != nil {
		return nil, err
	}

	ch.state = ichain.ChainState{
		StateSegment: seg.GetId(),
	}

	if err := ch.writeState(ctx); err != nil {
		return nil, err
	}

	go ch.manageState()
	return ch, nil
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
	dbm idb.Db,
	db *objstore.ObjectStore,
	conf *ichain.Config,
	blockValidator block.Validator,
) (*Chain, error) {
	le := logctx.GetLogEntry(ctx)
	if objstore.GetObjStore(ctx) == nil {
		ctx = objstore.WithObjStore(ctx, db)
	}

	encStrat, err := impl.BuildEncryptionStrategy(
		conf.GetEncryptionStrategy(),
		conf.GetEncryptionArgs(),
	)
	if err != nil {
		return nil, err
	}

	encConf := encStrat.GetGenesisEncryptionConfigWithDigest(conf.GetGenesisRef().GetObjectDigest())
	encConfCtx := pbobject.WithEncryptionConf(ctx, &encConf)

	genObj := &inca.Genesis{}
	if err := conf.GetGenesisRef().FollowRef(encConfCtx, nil, genObj); err != nil {
		return nil, errors.WithMessage(err, "cannot follow genesis reference")
	}

	ch := &Chain{
		ctx:                 ctx,
		conf:                conf,
		genesis:             genObj,
		db:                  db,
		dbm:                 dbm,
		encStrat:            encStrat,
		le:                  le,
		recheckStateTrigger: make(chan struct{}, 1),
		blockValidator:      blockValidator,
	}
	ch.segStore = segment.NewSegmentStore(
		ctx,
		idb.WithPrefix(dbm, []byte(fmt.Sprintf("/chain/%s/segments", ch.genesis.GetChainId()))),
		db,
		ch.GetEncryptionStrategy(),
		ch.GetBlockValidator(),
		ch.GetBlockDbm(),
	)
	ch.computePubsubTopic()

	if err := ch.readState(ctx); err != nil {
		return nil, errors.WithMessage(err, "cannot load state from db")
	}

	go ch.manageState()
	return ch, nil
}

// GetPubsubTopic returns the pubsub topic name.
func (c *Chain) GetPubsubTopic() string {
	return c.pubsubTopic
}

// GetConfig returns a copy of the chain config.
func (c *Chain) GetConfig() *ichain.Config {
	return proto.Clone(c.conf).(*ichain.Config)
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
}

// GetGenesisRef returns a copy of the chain genesis reference.
func (c *Chain) GetGenesisRef() *storageref.StorageRef {
	return proto.Clone(c.conf.GetGenesisRef()).(*storageref.StorageRef)
}

// GetBlockValidator returns the block validator.
func (c *Chain) GetBlockValidator() block.Validator {
	return c.blockValidator
}

// SetBlockValidator updates the block validator.
func (c *Chain) SetBlockValidator(bv block.Validator) {
	c.blockValidator = bv
}

// GetBlockProposer returns the block proposer, if set.
func (c *Chain) GetBlockProposer() block.Proposer {
	return c.blockProposer
}

// SetBlockProposer updates the block proposer.
func (c *Chain) SetBlockProposer(bp block.Proposer) {
	c.blockProposer = bp
}

// GetEncryptionStrategy returns the encryption strategy for this chain.
func (c *Chain) GetEncryptionStrategy() encryption.Strategy {
	return c.encStrat
}

// ValidateGenesisRef checks if the genesis references matches our local genesis reference.
func (c *Chain) ValidateGenesisRef(ref *storageref.StorageRef) error {
	if !ref.Equals(c.conf.GetGenesisRef()) {
		return errors.Errorf("genesis references do not match: %s (expected) != %s (actual)", c.conf.GetGenesisRef(), ref)
	}

	return nil
}

// GetBlockDbm returns the db used for blocks.
func (c *Chain) GetBlockDbm() idb.Db {
	return idb.WithPrefix(c.dbm, []byte("/blocks"))
}

// HandleBlockCommit handles an incoming block commit.
func (c *Chain) HandleBlockCommit(
	p *peer.Peer,
	blkRef *storageref.StorageRef,
	blk *inca.Block,
) error {
	ctx := c.ctx
	encStrat := c.GetEncryptionStrategy()
	blkDbm := c.GetBlockDbm()
	blkObj, err := block.GetBlock(
		ctx,
		encStrat,
		c.GetBlockValidator(),
		blkDbm,
		blkRef,
	)
	if err != nil {
		return err
	}

	// blkHeader := blkObj.GetHeader()
	blkSegID := blkObj.GetSegmentId()
	if blkSegID != "" {
		return nil
	}

	// Identify the parent of the block.
	// blkParentRef := blkHeader.GetLastBlockRef()
	blkHeader := blkObj.GetHeader()
	blkParentObj, err := block.GetBlock(
		ctx,
		encStrat,
		c.GetBlockValidator(),
		blkDbm,
		blkHeader.GetLastBlockRef(),
	)
	if err != nil {
		return err
	}

	// If the parent is identified...
	blkParentSegmentID := blkParentObj.GetSegmentId()
	if blkParentSegmentID != "" {
		blkParentSeg, err := c.segStore.GetSegmentById(ctx, blkParentSegmentID)
		if err != nil {
			return err
		}

		if err := blkParentSeg.AppendBlock(
			ctx,
			blkRef,
			blkObj,
			blkParentObj,
			c.GetEncryptionStrategy(),
		); err != nil {
			return err
		}

		c.triggerStateRecheck()
		return nil
	}

	// Create a new segment
	seg, err := c.segStore.NewSegment(ctx, blkObj, blkRef)
	if err != nil {
		return err
	}

	// c.SegmentStore.segmentQueue.Push(seg)
	_ = seg
	return nil
}

// dbKey returns the database key of this chain's state.
func (c *Chain) dbKey() []byte {
	return []byte(fmt.Sprintf("/chain/%s", c.genesis.GetChainId()))
}

// writeState writes the state to the database.
func (c *Chain) writeState(ctx context.Context) error {
	defer c.triggerStateRecheck()

	dat, err := proto.Marshal(&c.state)
	if err != nil {
		return err
	}

	return c.dbm.Set(ctx, c.dbKey(), dat)
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (c *Chain) readState(ctx context.Context) error {
	dat, err := c.dbm.Get(ctx, c.dbKey())
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return nil
	}

	return proto.Unmarshal(dat, &c.state)
}

// manageState manages the state
func (c *Chain) manageState() {
	ctx, ctxCancel := context.WithCancel(c.ctx)
	defer ctxCancel()

	// Main state update loop
	lastNextCheckTime := time.Now().Add(time.Second * 5)
	nextCheckTimer := time.NewTimer(time.Second * 5)
	var nextCheckCh <-chan time.Time
	for {
		if err := c.manageStateOnce(ctx); err != nil {
			c.le.WithError(err).Warn("chain management errored")
		}

		now := time.Now()
		nextCheckTime := lastNextCheckTime
		// Check at minimum every 10s
		if !nextCheckTime.After(now) {
			nextCheckTime = now.Add(time.Second * 10)
		}

		state := c.lastStateSnapshot
		if state != nil {
			if nextCheckTime.After(state.RoundEndTime) && state.RoundEndTime.After(now) {
				nextCheckTime = state.RoundEndTime
			}
			if nextCheckTime.After(state.NextRoundStartTime) && state.NextRoundStartTime.After(now) {
				nextCheckTime = state.NextRoundStartTime
			}
			if nextCheckTime.After(state.RoundStartTime) && state.RoundStartTime.After(now) {
				nextCheckTime = state.RoundStartTime
			}
		}

		if !nextCheckTime.Equal(lastNextCheckTime) {
			if !nextCheckTimer.Stop() {
				select {
				case <-nextCheckCh:
				default:
				}
			}

			nextCheckDur := nextCheckTime.Sub(now)
			nextCheckTimer.Reset(nextCheckDur)
			nextCheckCh = nextCheckTimer.C
			lastNextCheckTime = nextCheckTime
		}

		select {
		case <-ctx.Done():
			return
		case <-c.recheckStateTrigger:
		case <-nextCheckCh:
		}
	}
}

// manageStateOnce manages the state for one round.
func (c *Chain) manageStateOnce(ctx context.Context) error {
	// Evaluate current state
	stateSegmentID := c.state.GetStateSegment()
	if stateSegmentID == "" {
		return errors.New("no current state segment")
	}

	seg, err := c.segStore.GetSegmentById(ctx, stateSegmentID)
	if err != nil {
		return err
	}

	// Check if we should fast-forward the segment.
	if nextSegmentID := seg.GetSegmentNext(); nextSegmentID != "" {
		nextSegment, err := c.segStore.GetSegmentById(ctx, nextSegmentID)
		if err != nil {
			return err
		}

		c.state.StateSegment = nextSegmentID
		if err := c.writeState(ctx); err != nil {
			return err
		}

		seg = nextSegment
	}

	headBlockRef := seg.GetHeadBlock()
	headDigest := headBlockRef.GetObjectDigest()
	if bytes.Compare(headDigest, c.lastHeadDigest) != 0 {
		headBlock, err := block.FollowBlockRef(ctx, headBlockRef, c.encStrat)
		if err != nil {
			return err
		}

		headBlockHeader, err := block.FollowBlockHeaderRef(
			ctx,
			headBlock.GetBlockHeaderRef(),
			c.encStrat,
		)
		if err != nil {
			return err
		}

		now := time.Now()
		headBlockTs := headBlockHeader.GetBlockTs().ToTime()
		headStr := headBlockHeader.GetRoundInfo().String()
		if headStr == "" {
			headStr = "genesis"
		}
		c.le.
			WithField("head", headStr).
			WithField("block-ipfs-ref", headBlock.GetBlockHeaderRef().GetIpfs().GetReference()).
			WithField("block-ipfs-ref-type", headBlock.GetBlockHeaderRef().GetIpfs().GetIpfsRefType().String()).
			WithField("block-time-ago", now.Sub(headBlockTs).String()).
			Info("head block updated")
		c.lastBlock = headBlock
		c.lastBlockHeader = headBlockHeader
		c.lastBlockRef = headBlockRef
		c.lastHeadDigest = headDigest

		if err := c.writeState(ctx); err != nil {
			return err
		}
	}

	if err := c.computeEmitSnapshot(ctx); err != nil {
		return err
	}

	return nil
}

// computeEmitSnapshot computes the current state snapshot and emits it again if necessary.
func (c *Chain) computeEmitSnapshot(ctx context.Context) error {
	if c.lastBlockHeader == nil {
		return nil
	}

	lastHeight := c.state.GetLastHeight()
	lastRoundInfo := c.lastBlockHeader.GetRoundInfo()
	if lastRoundInfo.GetHeight()+1 > lastHeight {
		c.state.LastHeight = lastRoundInfo.GetHeight() + 1
		c.state.LastRound = 0
	}

	// compute expected round
	lastBlockTs := c.lastBlockHeader.GetBlockTs().ToTime()
	nowTs := time.Now()
	if lastBlockTs.After(nowTs) {
		return errors.New("last block was in the future")
	}
	sinceLastBlock := nowTs.Sub(lastBlockTs)

	chainConfig := &inca.ChainConfig{}
	{
		subCtx := c.GetEncryptContextWithDigest(
			ctx,
			c.lastBlockHeader.GetNextChainConfigRef().GetObjectDigest(),
			chainConfig,
		)
		err := c.lastBlockHeader.
			GetNextChainConfigRef().
			FollowRef(subCtx, nil, chainConfig)
		if err != nil {
			return err
		}
	}

	roundDuration := time.Millisecond * time.Duration(
		chainConfig.GetTimingConfig().GetRoundLength(),
	)
	if roundDuration < (1 * time.Second) {
		return errors.Errorf(
			"configured round duration too short: %s",
			roundDuration.String(),
		)
	}

	expectedRoundCount := uint64(sinceLastBlock / roundDuration)
	currRound := c.state.LastRound
	if expectedRoundCount > currRound {
		currRound = expectedRoundCount
	}

	roundStartDuration := time.Millisecond * time.Duration(
		chainConfig.GetTimingConfig().GetMinProposeAfterBlock(),
	)
	c.state.LastRound = currRound
	roundStartTime := lastBlockTs.Add(
		roundStartDuration + (roundDuration * time.Duration(currRound)),
	)
	roundEndTime := roundStartTime.Add(
		roundDuration,
	)
	nextRoundStartTime := roundEndTime.Add(roundDuration)
	currRoundInfo := &inca.BlockRoundInfo{
		Height: c.state.LastHeight,
		Round:  currRound,
	}

	// Compute current proposer
	validatorSet := &inca.ValidatorSet{}
	{
		subCtx := c.GetEncryptContextWithDigest(
			ctx,
			chainConfig.GetValidatorSetRef().GetObjectDigest(),
			validatorSet,
		)
		err := chainConfig.GetValidatorSetRef().FollowRef(subCtx, nil, validatorSet)
		if err != nil {
			return err
		}
	}

	validatorSet.SortValidators()
	proposer, powerSum := validatorSet.SelectProposer(
		c.lastHeadDigest,
		currRoundInfo.Height,
		currRoundInfo.Round,
	)

	currentSnap := &state.ChainStateSnapshot{
		BlockRoundInfo:     currRoundInfo,
		RoundStartTime:     roundStartTime,
		RoundEndTime:       roundEndTime,
		NextRoundStartTime: nextRoundStartTime,
		CurrentProposer:    proposer,
		LastBlockHeader:    c.lastBlockHeader,
		LastBlockRef:       c.lastBlockRef,
		TotalVotingPower:   powerSum,
		ChainConfig:        chainConfig,
		ValidatorSet:       validatorSet,
		RoundStarted:       roundStartTime.Before(time.Now()),
	}
	lastSnap := c.lastStateSnapshot
	if lastSnap != nil {
		// Detect if anything changed.
		switch {
		case lastSnap.BlockRoundInfo.Round != currRoundInfo.Round:
		case lastSnap.BlockRoundInfo.Height != currRoundInfo.Height:
		case !lastSnap.RoundEndTime.Equal(roundEndTime):
		case !lastSnap.RoundStartTime.Equal(roundStartTime):
		case currentSnap.RoundStarted != lastSnap.RoundStarted:
		case lastSnap.CurrentProposer == nil ||
			bytes.Compare(lastSnap.CurrentProposer.PubKey, proposer.GetPubKey()) != 0:
		default:
			return nil
		}
	}

	// Load blocks in as necessary.
	c.emitNextChainState(currentSnap)
	return nil
}

// GetEncryptContextWithDigest checks the type of obj an returns the encryption strategy ctx.
func (c *Chain) GetEncryptContextWithDigest(
	ctx context.Context,
	digest []byte,
	obj interface{},
) context.Context {
	var encConf pbobject.EncryptionConfig
	switch obj.(type) {
	case *inca.ValidatorSet:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfigWithDigest(digest)
	case *inca.ChainConfig:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfigWithDigest(digest)
	case *inca.BlockHeader:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfigWithDigest(digest)
	default:
		return nil
	}

	return pbobject.WithEncryptionConf(ctx, &encConf)
}

// GetEncryptContext checks the type of obj an returns the encryption strategy ctx.
func (c *Chain) GetEncryptContext(
	ctx context.Context,
	obj interface{},
) context.Context {
	var encConf pbobject.EncryptionConfig
	switch obj.(type) {
	case *inca.ValidatorSet:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfig()
	case *inca.ChainConfig:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfig()
	case *inca.BlockHeader:
		encConf = c.
			GetEncryptionStrategy().
			GetBlockEncryptionConfig()
	default:
		return nil
	}

	return pbobject.WithEncryptionConf(ctx, &encConf)
}

// triggerStateRecheck triggers a state recheck without blocking.
func (c *Chain) triggerStateRecheck() {
	select {
	case c.recheckStateTrigger <- struct{}{}:
	default:
	}
}

func (c *Chain) computePubsubTopic() {
	c.pubsubTopic = base64.StdEncoding.EncodeToString(c.conf.GetGenesisRef().GetObjectDigest())
}
