package chain

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/peer"
	"github.com/aperturerobotics/inca-go/segment"
	ichain "github.com/aperturerobotics/inca/chain"
	isegment "github.com/aperturerobotics/inca/segment"

	hblock "github.com/aperturerobotics/hydra/block"
	"github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/bucket"
	"github.com/aperturerobotics/hydra/bucket/event"
	"github.com/aperturerobotics/hydra/cid"
	hobject "github.com/aperturerobotics/hydra/object"
	"github.com/aperturerobotics/timestamp"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	lpeer "github.com/aperturerobotics/bifrost/peer"
	"github.com/libp2p/go-libp2p-crypto"
)

var genesisKey = "/genesis"

// Chain is an instance of a blockchain.
type Chain struct {
	ctx                 context.Context
	dbKey               string
	rootCursor          *object.Cursor
	store, blockStore   hobject.ObjectStore
	genesis             *inca.Genesis
	le                  *logrus.Entry
	state               ichain.ChainState
	recheckStateTrigger chan struct{}
	pubsubTopic         string

	stateSnapshotCtx       context.Context
	stateSnapshotCtxCancel context.CancelFunc
	lastStateSnapshot      *state.ChainStateSnapshot

	stateSubs       sync.Map
	lastHeadDigest  []byte
	lastBlock       *block.Block
	lastBlockHeader *inca.BlockHeader
	lastBlockRef    *cid.BlockRef
	segStore        *segment.SegmentStore
	genesisRef      *cid.BlockRef

	blockValidator block.Validator
	blockProposer  block.Proposer
}

// NewChain builds a new blockchain from scratch, minting a genesis block and
// committing it.
func NewChain(
	ctx context.Context,
	le *logrus.Entry,
	store hobject.ObjectStore,
	rootCursor *object.Cursor,
	bkt bucket.Bucket,
	chainID string,
	validatorPriv crypto.PrivKey,
	blockValidator block.Validator,
	genesisStateRef *cid.BlockRef,
) (ch *Chain, rerr error) {
	store = hobject.NewPrefixer(store, fmt.Sprintf("chain/%s/", chainID))
	if chainID == "" {
		return nil, errors.New("chain id must be set")
	}

	validatorPub := validatorPriv.GetPublic()
	validatorPubBytes, err := crypto.MarshalPublicKey(validatorPub)
	if err != nil {
		return nil, err
	}

	validatorID, err := lpeer.IDFromPublicKey(validatorPub)
	if err != nil {
		return nil, err
	}

	tx, cursor := rootCursor.BuildTransaction(nil)
	mintTs := timestamp.Now()
	b, err := crypto.MarshalPublicKey(validatorPub)
	if err != nil {
		return nil, err
	}
	proposerIdx := 0
	vsetObj := &inca.ValidatorSet{
		Validators: []*inca.Validator{
			&inca.Validator{
				PubKey:        b,
				VotingPower:   10,
				OperationMode: inca.Validator_OperationMode_OPERATING,
			},
		},
	}
	chainConfObj := &inca.ChainConfig{
		TimingConfig: &inca.TimingConfig{
			MinProposeAfterBlock: 2000,
			RoundLength:          500,
		},
		// ValidatorSetRef is set by reference.
	}
	genesisObj := &inca.Genesis{
		ChainId:   chainID,
		Timestamp: &mintTs,
		// InitChainConfigRef is set by reference.
	}
	blockHeaderObj := &inca.BlockHeader{
		// GenesisRef is set by reference.
		// ChainConfigRef is set by refernece.
		RoundInfo:      &inca.BlockRoundInfo{},
		BlockTs:        &mintTs,
		ValidatorIndex: uint32(proposerIdx),
		StateRef:       genesisStateRef,
	}
	cursor.SetBlock(blockHeaderObj)

	genesisCursor, err := cursor.FollowRef(1, nil)
	if err != nil {
		return nil, err
	}
	genesisCursor.SetBlock(genesisObj)
	cursor.SetPreWriteHook(func(b hblock.Block) error {
		b.(*inca.BlockHeader).ChainConfigRef = genesisObj.GetInitChainConfigRef()
		return nil
	})

	chainConfCursor, err := genesisCursor.FollowRef(3, nil)
	if err != nil {
		return nil, err
	}
	chainConfCursor.SetBlock(chainConfObj)

	validatorSetCursor, err := chainConfCursor.FollowRef(2, nil)
	if err != nil {
		return nil, err
	}
	validatorSetCursor.SetBlock(vsetObj)

	// purgeBlocks purges a set of blocks
	purgeBlocks := func(eves []*bucket_event.Event) {
		for _, e := range eves {
			if e.GetEventType() == bucket_event.EventType_EventType_PUT_BLOCK {
				_ = bkt.RmBlock(e.GetPutBlock().GetBlockCommon().GetBlockRef())
			}
		}
	}

	// Compute the block header reference
	blockRefs, cursor, err := tx.Write()
	if err != nil {
		return nil, err
	}
	defer func() {
		if rerr != nil {
			purgeBlocks(blockRefs)
		}
	}()

	blockHeaderRef := blockRefs[len(blockRefs)-1].
		GetPutBlock().
		GetBlockCommon().
		GetBlockRef()

	// Sign the block header
	voteObj := &inca.Vote{
		// BlockHeaderRef is set by refence.
		ValidatorIndex: uint32(proposerIdx),
	}
	if err := voteObj.SignBlockHeader(
		validatorPriv,
		blockHeaderRef,
	); err != nil {
		return nil, err
	}

	blockObj := &inca.Block{
		BlockHeaderRef: blockHeaderRef,
	}
	cursor.SetBlock(blockObj)
	voteCursor, err := cursor.FollowRef(2, nil)
	if err != nil {
		return nil, err
	}
	voteCursor.SetBlock(voteObj)
	rblockRefs, cursor, err := tx.Write()
	if err != nil {
		return nil, err
	}
	firstBlkRef := rblockRefs[len(rblockRefs)-1].
		GetPutBlock().
		GetBlockCommon().
		GetBlockRef()

	blockStore := hobject.NewPrefixer(store, "blocks/")
	ch = &Chain{
		ctx:        ctx,
		genesis:    genesisObj,
		store:      store,
		blockStore: blockStore,
		le:         le,

		blockValidator:      blockValidator,
		recheckStateTrigger: make(chan struct{}, 1),
	}

	ssStore := hobject.NewPrefixer(
		store,
		"segments/",
	)
	ch.segStore = segment.NewSegmentStore(
		ctx,
		ssStore,
		rootCursor,
		ch.GetBlockValidator(),
		ch,
	)
	ch.computePubsubTopic()

	_, firstBlkCursor := rootCursor.BuildTransactionAtRef(nil, firstBlkRef)
	firstBlk, err := block.GetBlock(
		ctx,
		firstBlkCursor,
		ch.GetBlockDbm(),
	)
	if err != nil {
		return nil, err
	}

	seg, err := ch.segStore.NewSegment(ctx, firstBlk, firstBlkRef)
	if err != nil {
		return nil, err
	}

	firstBlk.State.SegmentId = seg.GetId()
	if err := firstBlk.WriteState(blockStore); err != nil {
		return nil, err
	}

	ch.state = ichain.ChainState{
		StateSegment: seg.GetId(),
	}

	if err := ch.writeState(ctx); err != nil {
		return nil, err
	}

	ch.lastBlockHeader = blockHeaderObj
	ch.lastBlockRef = firstBlkRef
	ch.lastBlock = firstBlk
	ch.genesisRef = firstBlk.GetHeader().GetGenesisRef()
	ch.lastHeadDigest, err = firstBlkRef.MarshalKey()
	if err != nil {
		return nil, err
	}

	go ch.manageState()
	return ch, nil
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
	le *logrus.Entry,
	chainID string,
	store hobject.ObjectStore,
	bkt bucket.Bucket,
	rootCursor *object.Cursor,
	genesisRef *cid.BlockRef,
	blockValidator block.Validator,
) (*Chain, error) {
	genTx, genCursor := rootCursor.BuildTransactionAtRef(nil, genesisRef)
	geni, err := genCursor.Unmarshal(func() hblock.Block { return &inca.Genesis{} })
	if err != nil {
		return nil, err
	}
	genObj := geni.(*inca.Genesis)
	if genObj.GetChainId() != chainID {
		return nil, errors.Errorf("chain id mismatch: %s != %s", chainID, genObj.GetChainId())
	}

	store = hobject.NewPrefixer(store, fmt.Sprintf("chain/%s/", chainID))
	ch := &Chain{
		le:                  le,
		ctx:                 ctx,
		genesis:             genObj,
		genesisRef:          genesisRef,
		store:               store,
		blockStore:          hobject.NewPrefixer(store, "blocks/"),
		recheckStateTrigger: make(chan struct{}, 1),
		blockValidator:      blockValidator,
	}
	ch.segStore = segment.NewSegmentStore(
		ctx,
		hobject.NewPrefixer(store, "segments/"),
		rootCursor,
		ch.GetBlockValidator(),
		ch,
	)
	ch.computePubsubTopic()

	if err := ch.readState(ctx); err != nil {
		return nil, errors.WithMessage(err, "cannot load state from db")
	}

	go ch.manageState()
	return ch, nil
}

// GetContext returns the chain context.
// This context is bound to the chain object store.
func (c *Chain) GetContext() context.Context {
	return c.ctx
}

// GetPubsubTopic returns the pubsub topic name.
func (c *Chain) GetPubsubTopic() string {
	return c.pubsubTopic
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
}

// GetGenesisRef returns a copy of the chain genesis reference.
func (c *Chain) GetGenesisRef() *cid.BlockRef {
	return proto.Clone(c.genesisRef).(*cid.BlockRef)
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

// ValidateGenesisRef checks if the genesis references matches our local genesis reference.
func (c *Chain) ValidateGenesisRef(ref *cid.BlockRef) error {
	if !ref.EqualsRef(c.genesisRef) {
		return errors.Errorf(
			"genesis references do not match: %s (expected) != %s (actual)",
			c.genesisRef.MarshalString(),
			ref.MarshalString(),
		)
	}

	return nil
}

// GetBlockDbm returns the db used for blocks.
func (c *Chain) GetBlockDbm() hobject.ObjectStore {
	return c.blockStore
}

// GetHeadBlock returns the current head block.
func (c *Chain) GetHeadBlock() *block.Block {
	return c.lastBlock
}

// HandleBlockCommit handles an incoming block commit.
func (c *Chain) HandleBlockCommit(
	p *peer.Peer,
	blkRef *cid.BlockRef,
	blk *inca.Block,
) error {
	ctx := c.ctx
	blkDbm := c.GetBlockDbm()
	_, blkCursor := c.rootCursor.BuildTransactionAtRef(nil, blkRef)
	blkObj, err := block.GetBlock(
		ctx,
		blkCursor,
		blkDbm,
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
	lastBlkCursor, err := blkCursor.FollowRef(
		4,
		blkHeader.GetPrevBlockRef(),
	)
	if err != nil {
		return err
	}
	blkParentObj, err := block.GetBlock(
		ctx,
		lastBlkCursor,
		blkDbm,
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
			c.blockValidator,
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

	_ = seg
	return nil
}

// writeState writes the state to the database.
func (c *Chain) writeState(ctx context.Context) error {
	defer c.triggerStateRecheck()

	dat, err := proto.Marshal(&c.state)
	if err != nil {
		return err
	}

	return c.store.SetObject(c.dbKey, dat)
}

var stateKey = "state"

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (c *Chain) readState(ctx context.Context) error {
	dat, datOk, err := c.store.GetObject(stateKey)
	if err != nil || !datOk {
		return err
	}

	return proto.Unmarshal(dat, &c.state)
}

// manageState manages the state
func (c *Chain) manageState() {
	ctx, ctxCancel := context.WithCancel(c.ctx)
	defer ctxCancel()

	// Segment queue rewind
	go c.manageSegmentRewind(ctx)

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

// manageSegmentRewind manages the segment rewind queue.
func (c *Chain) manageSegmentRewind(ctx context.Context) {
	for {
		matchedAny := c.segStore.RewindOnce(
			ctx,
			c.GetBlockValidator(),
			c.GetBlockDbm(),
		)

		if matchedAny {
			continue
		}

		select {
		case <-time.After(time.Millisecond * 500):
		case <-ctx.Done():
			return
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
	headDigest, err := headBlockRef.MarshalKey()
	if err != nil {
		return err
	}
	_, headBlockCursor := c.rootCursor.BuildTransactionAtRef(nil, headBlockRef)
	headBlock, err := block.GetBlock(c.ctx, headBlockCursor, c.GetBlockDbm())
	if err != nil {
		return errors.Wrap(err, "lookup head block")
	}

	if bytes.Compare(headDigest, c.lastHeadDigest) != 0 {
		if err := c.setHeadBlock(ctx, headBlock); err != nil {
			return err
		}
	}

	if err := c.computeEmitSnapshot(ctx); err != nil {
		return err
	}

	return nil
}

// setHeadBlock updates the head block.
func (c *Chain) setHeadBlock(
	ctx context.Context,
	headBlock *block.Block,
) error {
	headBlockHeaderRef := headBlock.GetInnerBlock().GetBlockHeaderRef()
	headBlockHeader := headBlock.GetHeader()

	now := time.Now()
	headBlockTs := headBlockHeader.GetBlockTs().ToTime()
	headStr := headBlockHeader.GetRoundInfo().String()
	headDigest, err := headBlock.GetBlockRef().MarshalKey()
	if err != nil {
		return err
	}
	if headStr == "" {
		headStr = "genesis"
	}
	c.le.
		WithField("head", headStr).
		WithField("block-segment", headBlock.GetSegmentId()).
		WithField("block-ipfs-ref", headBlockHeaderRef.MarshalString()).
		WithField("block-ipfs-ref-type", headBlockHeaderRef.MarshalString()).
		WithField("block-time-ago", now.Sub(headBlockTs).String()).
		Debug("head block updated")

	c.lastBlock = headBlock
	c.lastBlockHeader = headBlockHeader
	c.lastBlockRef = headBlock.GetBlockRef()
	c.lastHeadDigest = headDigest

	c.state.StateSegment = headBlock.GetSegmentId()
	c.state.LastHeight = headBlock.GetHeader().GetRoundInfo().GetHeight() + 1
	c.state.LastRound = 0
	return c.writeState(ctx)
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

	nextCcRef := c.lastBlockHeader.GetNextChainConfigRef()
	_, nextCcCursor := c.rootCursor.BuildTransactionAtRef(nil, nextCcRef)
	chainConfigi, err := nextCcCursor.Unmarshal(func() hblock.Block { return &inca.ChainConfig{} })
	if err != nil {
		return err
	}
	chainConfig := chainConfigi.(*inca.ChainConfig)

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
	validatorSetRef := chainConfig.GetValidatorSetRef()
	validatorSetCursor, err := nextCcCursor.FollowRef(2, validatorSetRef)
	if err != nil {
		return err
	}
	validatorSeti, err := validatorSetCursor.Unmarshal(
		func() hblock.Block { return &inca.ValidatorSet{} },
	)
	if err != nil {
		return err
	}
	validatorSet := validatorSeti.(*inca.ValidatorSet)

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

// GetBlock returns a block in the chain.
func (c *Chain) GetBlock(blockRef *cid.BlockRef) (*block.Block, error) {
	_, cursor := c.rootCursor.BuildTransactionAtRef(nil, blockRef)
	return block.GetBlock(c.ctx, cursor, c.blockStore)
}

// HandleSegmentUpdated handles a segment store update.
func (c *Chain) HandleSegmentUpdated(seg *segment.Segment) {
	segStatus := seg.GetStatus()
	segDisjointed := segStatus == isegment.SegmentStatus_SegmentStatus_DISJOINTED
	if !segDisjointed || c.GetHeadBlock() != nil {
		return
	}

	blkDbm := c.GetBlockDbm()
	tailBlock := seg.GetTailBlock()
	headBlock := seg.GetHeadBlock()
	tailRound := seg.GetTailBlockRound()
	tailRoundGenesis := tailRound.GetHeight() == 0

	_, headBlkCursor := c.rootCursor.BuildTransactionAtRef(nil, headBlock)
	headBlkObj, err := block.GetBlock(
		c.ctx,
		headBlkCursor,
		blkDbm,
	)
	if err != nil {
		c.le.WithError(err).Warn("unable to lookup head block")
		return
	}

	_, tailBlkCursor := c.rootCursor.BuildTransactionAtRef(nil, tailBlock)
	tailBlkObj, err := block.GetBlock(
		c.ctx,
		tailBlkCursor,
		blkDbm,
	)
	if err != nil {
		c.le.WithError(err).Warn("unable to lookup tail block")
		return
	}

	chainConfMatches := tailBlkObj.
		GetHeader().
		GetChainConfigRef().
		EqualsRef(
			c.GetGenesis().GetInitChainConfigRef(),
		)

	markAsHead := func() {
		if err := seg.MarkValid(); err != nil {
			c.le.WithError(err).Warn("unable to mark segment as valid")
			return
		}
		if err := c.setHeadBlock(c.ctx, headBlkObj); err != nil {
			c.le.WithError(err).Warn("unable to mark segment as new head block")
		}
	}

	// Validate block via reaching genesis block.
	if tailRoundGenesis {
		if chainConfMatches {
			c.le.Info("traversed to genesis block, marking segment as valid")
			markAsHead()
		} else {
			c.le.Info("traversed to invalid block, not marking segment as valid")
		}

		return
	}

	// Validate block via same validator set
	if chainConfMatches {
		markAsHead()
		return
	}

	initChainConfRef := c.GetGenesis().GetInitChainConfigRef()
	_, initChainConfCursor := c.rootCursor.BuildTransactionAtRef(nil, initChainConfRef)
	initChainConfi, err := initChainConfCursor.Unmarshal(func() hblock.Block {
		return &inca.ChainConfig{}
	})
	if err != nil {
		c.le.WithError(err).Warn("cannot fetch initial chain conifg")
		return
	}
	initChainConf := initChainConfi.(*inca.ChainConfig)

	blkChainConfRef := headBlkObj.GetHeader().GetChainConfigRef()
	_, blkChainConfCursor := c.rootCursor.BuildTransactionAtRef(nil, blkChainConfRef)
	blkChainConfi, err := initChainConfCursor.Unmarshal(func() hblock.Block {
		return &inca.ChainConfig{}
	})
	if err != nil {
		c.le.WithError(err).Warn("cannot fetch block chain conifg")
		return
	}
	blkChainConf := blkChainConfi.(*inca.ChainConfig)

	if initChainConf.GetValidatorSetRef().EqualsRef(blkChainConf.GetValidatorSetRef()) {
		markAsHead()
	}
}

// triggerStateRecheck triggers a state recheck without blocking.
func (c *Chain) triggerStateRecheck() {
	select {
	case c.recheckStateTrigger <- struct{}{}:
	default:
	}
}

func (c *Chain) computePubsubTopic() {
	c.pubsubTopic = c.genesisRef.MarshalString()
}
