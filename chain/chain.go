package chain

import (
	"context"
	"encoding/base64"
	"path"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/peer"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/aperturerobotics/timestamp"
	"github.com/golang/protobuf/proto"
	api "github.com/ipfs/go-ipfs-api"
	"github.com/pkg/errors"
)

var genesisKey = "/genesis"

// Chain is an instance of a block chain.
type Chain struct {
	ctx       context.Context
	db        *objstore.ObjectStore
	conf      *Config
	genesis   *inca.Genesis
	encStrat  encryption.Strategy
	le        *logrus.Entry
	sh        *api.Shell
	peerStore *peer.PeerStore
}

// applyPrefix applies the chain ID prefix to the key.
func (c *Chain) applyPrefix(key string) []byte {
	return []byte(path.Join("/chain", c.genesis.GetChainId(), key))
}

// NewChain builds a new blockchain from scratch, minting a genesis block and committing it to IPFS.
func NewChain(
	ctx context.Context,
	dbm db.Db,
	db *objstore.ObjectStore,
	chainID string,
	sh *api.Shell,
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

	peerStore := peer.NewPeerStore(ctx, dbm, db, storageRef.GetObjectDigest())
	ch := &Chain{
		ctx:       ctx,
		conf:      conf,
		genesis:   genesis,
		db:        db,
		sh:        sh,
		le:        le,
		peerStore: peerStore,
	}
	conf.EncryptionArgs = argsObjectWrapper
	ch.encStrat = strat
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
	peerStore := peer.NewPeerStore(ctx, dbm, db, conf.GetGenesisRef().GetObjectDigest())
	return &Chain{
		ctx:       ctx,
		conf:      conf,
		genesis:   genObj,
		db:        db,
		encStrat:  encStrat,
		le:        le,
		peerStore: peerStore,
	}, nil
}

// manageChain manages a chain.
func (c *Chain) manageChain() {
	for {
		err := c.manageChainOnce()
		if err != nil && err != context.Canceled {
			c.le.WithError(err).Warn("chain exited with error")
		}

		select {
		case <-time.After(time.Second * 1):
		case <-c.ctx.Done():
			return
		}
	}
}

// manageChainOnce attempts to manage once, returning any transient errors
func (c *Chain) manageChainOnce() error {
	ctx, ctxCancel := context.WithCancel(c.ctx)
	defer ctxCancel()

	topicName := base64.StdEncoding.EncodeToString(c.conf.GetGenesisRef().GetObjectDigest())
	sub, err := c.sh.PubSubSubscribe(topicName)
	if err != nil {
		return err
	}
	defer sub.Cancel()

	errCh := make(chan error, 5)
	go func() {
		errCh <- c.listenPubSub(ctx, sub)
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			if err != nil {
				return err
			}
		}
	}
}

// listenPubSub processes pubsub messages
func (c *Chain) listenPubSub(ctx context.Context, sub *api.PubSubSubscription) error {
	for {
		nr, err := sub.Next()
		if err != nil {
			return err
		}

		// marked maybe because this can be easily falsified in the current implementation
		l := c.le.WithField("maybe-from-peer", nr.From().Pretty()).WithField("len", len(nr.Data()))
		l.Debug("processing pubsub message")

		if err := c.processPubsubMessage(ctx, nr.Data()); err != nil {
			l.WithError(err).Warn("ignoring pubsub message")
			continue
		}
	}
}

// processPubsubMessage processes a pubsub message.
func (c *Chain) processPubsubMessage(ctx context.Context, data []byte) error {
	// the data in a pubsub message is expected to be a storageref
	ref := &storageref.StorageRef{}
	if err := proto.Unmarshal(data, ref); err != nil {
		return err
	}

	// Follow the StorageRef to the NodeMessage
	encConf := c.encStrat.GetNodeMessageEncryptionConfigWithDigest(ref.GetObjectDigest())
	encCtx := pbobject.WithEncryptionConf(ctx, &encConf)
	nodeMessage := &inca.NodeMessage{}
	if err := ref.FollowRef(encCtx, nil, nodeMessage); err != nil {
		return err
	}

	return nil
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
