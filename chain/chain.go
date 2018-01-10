package chain

import (
	"context"
	"path"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/shell"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
)

var genesisKey = "/genesis"

// Chain is an instance of a block chain.
type Chain struct {
	db      db.Db
	conf    *Config
	shell   *shell.Shell
	genesis *inca.Genesis
}

// applyPrefix applies the chain ID prefix to the key.
func (c *Chain) applyPrefix(key string) string {
	return path.Join(c.genesis.GetChainId(), key)
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	db db.Db,
	shell *shell.Shell,
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

	tag, err := shell.AddProtobufObject(ctx, genesis)
	if err != nil {
		return nil, err
	}

	conf := &Config{
		GenesisRef: storageref.NewStorageRefIPFS(tag),
	}
	ch := &Chain{conf: conf, shell: shell, genesis: genesis, db: db}
	if err := ch.DbWriteGenesis(ctx); err != nil {
		return nil, err
	}

	return ch, nil
}

// DbWriteGenesis writes the genesis document to the db.
func (c *Chain) DbWriteGenesis(ctx context.Context) error {
	key := c.applyPrefix(genesisKey)
	return c.db.Set(ctx, key, c.genesis)
}

// FromConfig loads a blockchain from a config.
func FromConfig(
	ctx context.Context,
	db db.Db,
	shell *shell.Shell,
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

	return &Chain{conf: conf, shell: shell, genesis: gen, db: db}, nil
}

// GetConfig returns a copy of the chain config.
func (c *Chain) GetConfig() *Config {
	return proto.Clone(c.conf).(*Config)
}

// GetGenesis returns a copy of the genesis.
func (c *Chain) GetGenesis() *inca.Genesis {
	return proto.Clone(c.genesis).(*inca.Genesis)
}
