package chain

import (
	"context"
	"path"

	"github.com/aperturerobotics/inca"
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
	db      *objstore.ObjectStore
	conf    *Config
	genesis *inca.Genesis
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

	genesisTs := timestamp.Now()
	genesis := &inca.Genesis{
		ChainId:   chainID,
		Timestamp: &genesisTs,
	}

	// TODO: encryption config
	storageRef, _, err := db.StoreObject(ctx, genesis, pbobject.EncryptionConfig{})
	if err != nil {
		return nil, err
	}

	conf := &Config{
		GenesisRef: storageref.NewStorageRefIPFS(storageRef),
	}
	ch := &Chain{conf: conf, genesis: genesis, db: db}
	return ch, nil
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
	db *objstore.ObjectStore,
	conf *Config,
) (*Chain, error) {
	genObj, err := conf.GetGenesisRef().FollowRef(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, "cannot follow genesis reference")
	}

	gen, ok := genObj.(*inca.Genesis)
	if !ok {
		return nil, errors.Errorf("genesis object unrecognized: %s", genObj.GetObjectTypeID().GetTypeUuid())
	}

	return &Chain{conf: conf, genesis: gen, db: db}, nil
}

// GetConfig returns a copy of the chain config.
func (c *Chain) GetConfig() *Config {
	return proto.Clone(c.conf).(*Config)
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
}

// ValidateGenesisRef checks if the genesis references matches our local genesis reference.
func (c *Chain) ValidateGenesisRef(ref *storageref.StorageRef) error {
	if !ref.Equals(c.conf.GetGenesisRef()) {
		return errors.Errorf("genesis references do not match: %s (expected) != %s (actual)", c.conf.GetGenesisRef(), ref)
	}

	return nil
}
