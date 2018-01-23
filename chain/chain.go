package chain

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	idb "github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	srdigest "github.com/aperturerobotics/storageref/digest"
	_ "github.com/aperturerobotics/storageref/ipfs"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

var genesisKey = "/genesis"

// Chain is an instance of a blockchain.
type Chain struct {
	*SegmentStore
	ctx                 context.Context
	db                  *objstore.ObjectStore
	dbm                 idb.Db
	conf                *Config
	genesis             *inca.Genesis
	encStrat            encryption.Strategy
	le                  *logrus.Entry
	state               ChainState
	recheckStateTrigger chan struct{}
	pubsubTopic         string

	stateSnapshotCtx       context.Context
	stateSnapshotCtxCancel context.CancelFunc
	lastStateSnapshot      *ChainStateSnapshot

	stateSubs       sync.Map
	lastHeadDigest  []byte
	lastBlock       *inca.Block
	lastBlockHeader *inca.BlockHeader
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	dbm idb.Db,
	db *objstore.ObjectStore,
	chainID string,
	validatorPriv crypto.PrivKey,
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

	genesisTs := timestamp.Now()
	genesis := &inca.Genesis{
		ChainId:     chainID,
		Timestamp:   &genesisTs,
		EncStrategy: strat.GetEncryptionStrategyType(),
	}

	genesisStorageRef, _, err := db.StoreObject(
		ctx,
		genesis,
		strat.GetGenesisEncryptionConfig(),
	)
	if err != nil {
		return nil, err
	}

	conf := &Config{
		GenesisRef:         genesisStorageRef,
		EncryptionStrategy: strat.GetEncryptionStrategyType(),
	}

	ch := &Chain{
		ctx:                 ctx,
		conf:                conf,
		genesis:             genesis,
		db:                  db,
		dbm:                 dbm,
		le:                  le,
		recheckStateTrigger: make(chan struct{}, 1),
	}
	ch.SegmentStore = NewSegmentStore(ctx, ch, idb.WithPrefix(dbm, []byte(fmt.Sprintf("/chain/%s/segments", ch.genesis.GetChainId()))))
	conf.EncryptionArgs = argsObjectWrapper
	ch.encStrat = strat
	ch.computePubsubTopic()

	validatorPub := validatorPriv.GetPublic()
	validatorPubBytes, err := validatorPub.Bytes()
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
			MaxProposeAfterBlock: 10000,
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
		BlockTs: &nowTs,
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
	firstSeg := &SegmentState{
		Id:        uuid.NewV4().String(),
		Status:    SegmentStatus_SegmentStatus_VALID,
		HeadBlock: firstBlockStorageRef,
		TailBlock: firstBlockStorageRef,
	}
	if err := db.LocalStore.StoreLocal(ctx, firstSeg, &firstSegDigest, objstore.StoreParams{}); err != nil {
		return nil, err
	}

	ch.state = ChainState{
		StateSegmentRef: srdigest.NewStorageRefDigest(firstSegDigest),
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
	conf *Config,
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
	}
	ch.SegmentStore = NewSegmentStore(ctx, ch, idb.WithPrefix(dbm, []byte(fmt.Sprintf("/chain/%s/segments", ch.genesis.GetChainId()))))
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
func (c *Chain) GetConfig() *Config {
	return proto.Clone(c.conf).(*Config)
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
}

// GetGenesisRef returns a copy of the chain genesis reference.
func (c *Chain) GetGenesisRef() *storageref.StorageRef {
	return proto.Clone(c.conf.GetGenesisRef()).(*storageref.StorageRef)
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

// GetObjectTypeID returns the object type string, used to identify types.
func (g *ChainState) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/chain-state")
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
	for {
		if err := c.manageStateOnce(ctx); err != nil {
			c.le.WithError(err).Warn("chain management errored")
		}

		select {
		case <-ctx.Done():
			return
		case <-c.recheckStateTrigger:
		}
	}
}

func (c *Chain) manageStateOnce(ctx context.Context) error {
	// Evaluate current state
	stateSegment := c.state.GetStateSegmentRef()
	if stateSegment == nil {
		return errors.New("no current state segment")
	}

	segmentState := &SegmentState{}
	if err := stateSegment.FollowRef(ctx, nil, segmentState); err != nil {
		return err
	}

	// Check if we should fast-forward the segment.
	if segmentState.GetSegmentNext() != nil {
		nextSegmentState := &SegmentState{}
		if err := segmentState.GetSegmentPrev().FollowRef(ctx, nil, nextSegmentState); err != nil {
			return err
		}

		c.state.StateSegmentRef = segmentState.GetSegmentNext()
		if err := c.writeState(ctx); err != nil {
			return err
		}

		segmentState = nextSegmentState
	}

	headDigest := segmentState.GetHeadBlock().GetObjectDigest()
	if bytes.Compare(headDigest, c.lastHeadDigest) != 0 {
		headBlock := &inca.Block{}
		{
			encConf := c.GetEncryptionStrategy().GetBlockEncryptionConfigWithDigest(segmentState.GetHeadBlock().GetObjectDigest())
			ctx := pbobject.WithEncryptionConf(ctx, &encConf)
			if err := segmentState.GetHeadBlock().FollowRef(ctx, nil, headBlock); err != nil {
				return err
			}
		}

		headBlockHeader := &inca.BlockHeader{}
		{
			encConf := c.GetEncryptionStrategy().GetBlockEncryptionConfigWithDigest(headBlock.GetBlockHeaderRef().GetObjectDigest())
			ctx := pbobject.WithEncryptionConf(ctx, &encConf)
			if err := headBlock.GetBlockHeaderRef().FollowRef(ctx, nil, headBlockHeader); err != nil {
				return err
			}
		}

		now := time.Now()
		headBlockTs := headBlockHeader.GetBlockTs().ToTime()
		c.le.
			WithField("head", headBlockHeader.GetRoundInfo().String()).
			WithField("block-ipfs-ref", headBlock.GetBlockHeaderRef().GetIpfs().GetObjectHash()).
			WithField("block-time-ago", now.Sub(headBlockTs).String()).
			Debug("head block updated")
		c.lastBlock = headBlock
		c.lastBlockHeader = headBlockHeader

		// TODO: update blockchain state, emit state snapshot update
		// Compute current height and round and when the round will end
		// stateSnap := &ChainStateSnapshot{}
	}

	return nil
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
