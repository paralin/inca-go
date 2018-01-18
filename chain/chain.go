package chain

import (
	"context"
	"path"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

var genesisKey = "/genesis"

// Chain is an instance of a block chain.
type Chain struct {
	db       *objstore.ObjectStore
	conf     *Config
	genesis  *inca.Genesis
	encStrat encryption.Strategy
}

// applyPrefix applies the chain ID prefix to the key.
func (c *Chain) applyPrefix(key string) []byte {
	return []byte(path.Join("/chain", c.genesis.GetChainId(), key))
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	db *objstore.ObjectStore,
	chainID string,
) (*Chain, error) {
	if chainID == "" {
		return nil, errors.New("chain id must be set")
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
		ChainId:   chainID,
		Timestamp: &genesisTs,
	}

	// TODO: encryption config
	storageRef, _, err := db.StoreObject(ctx, genesis, strat.GetGenesisEncryptionConfig())
	if err != nil {
		return nil, err
	}

	conf := &Config{
		GenesisRef:         storageRef,
		EncryptionStrategy: convergentimmutable.StrategyType,
	}

	ch := &Chain{conf: conf, genesis: genesis, db: db}
	conf.EncryptionArgs = argsObjectWrapper
	ch.encStrat = strat
	return ch, nil
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
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

	return &Chain{conf: conf, genesis: genObj, db: db, encStrat: encStrat}, nil
}

// GetConfig returns a copy of the chain config.
func (c *Chain) GetConfig() *Config {
	return proto.Clone(c.conf).(*Config)
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
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
