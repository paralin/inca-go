package chain

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	srdigest "github.com/aperturerobotics/storageref/digest"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
	"github.com/satori/go.uuid"
)

var genesisKey = "/genesis"

// Chain is an instance of a blockchain.
type Chain struct {
	ctx      context.Context
	db       *objstore.ObjectStore
	dbm      db.Db
	conf     *Config
	genesis  *inca.Genesis
	encStrat encryption.Strategy
	le       *logrus.Entry
	state    ChainState
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	dbm db.Db,
	db *objstore.ObjectStore,
	chainID string,
	validatorPriv crypto.PrivKey,
) (*Chain, error) {
	if chainID == "" {
		return nil, errors.New("chain id must be set")
	}

	le := logctx.GetLogEntry(ctx)
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
		ctx:     ctx,
		conf:    conf,
		genesis: genesis,
		db:      db,
		dbm:     dbm,
		le:      le,
	}
	conf.EncryptionArgs = argsObjectWrapper
	ch.encStrat = strat

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
	firstBlockEncConf := strat.GetBlockEncryptionConfig()
	firstBlockEncConf.SignerKeys = []crypto.PrivKey{validatorPriv}
	firstBlock := &inca.BlockHeader{
		GenesisRef:         genesisStorageRef,
		ChainConfigRef:     chainConfStorageRef,
		NextChainConfigRef: chainConfStorageRef,
		RoundInfo: &inca.BlockRoundInfo{
			Height: 1,
			Round:  1,
		},
	}
	firstBlockStorageRef, _, err := db.StoreObject(
		ctx,
		firstBlock,
		firstBlockEncConf,
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

	return ch, nil
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
	dbm db.Db,
	db *objstore.ObjectStore,
	conf *Config,
) (*Chain, error) {
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

	le := logctx.GetLogEntry(ctx)
	ch := &Chain{
		ctx:      ctx,
		conf:     conf,
		genesis:  genObj,
		db:       db,
		dbm:      dbm,
		encStrat: encStrat,
		le:       le,
	}
	if err := ch.readState(ctx); err != nil {
		return nil, errors.WithMessage(err, "cannot load state from db")
	}
	return ch, nil
}

// GetPubsubTopic returns the pubsub topic name.
func (c *Chain) GetPubsubTopic() string {
	return base64.StdEncoding.EncodeToString(c.conf.GetGenesisRef().GetObjectDigest())
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
