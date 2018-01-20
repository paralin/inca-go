package convergentimmutable

import (
	"context"
	"crypto/rand"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/objectenc/secretbox"
	"github.com/aperturerobotics/objstore/localdb"
	"github.com/aperturerobotics/pbobject"
	"github.com/pkg/errors"
)

// StrategyType is the strategy type of this implementation.
const StrategyType = inca.EncryptionStrategy_EncryptionStrategy_ConvergentImmutable

// EncType is the encryption type of this implementation.
const EncType = objectenc.EncryptionType_EncryptionType_SECRET_BOX

// hack: directly call local db compliant digest
var sampleLocalDb = &localdb.LocalDb{}

// Strategy implements the ConvergentImmutable strategy.
type Strategy struct {
	Key  [32]byte
	conf *Config
}

// NewConvergentImmutableStrategyFromConfig builds a new convergent immutable strategy.
func NewConvergentImmutableStrategyFromConfig(conf *pbobject.ObjectWrapper) (encryption.Strategy, error) {
	s := &Strategy{}

	sc := &Config{}
	if err := conf.DecodeToObject(sc, pbobject.EncryptionConfig{}); err != nil {
		return nil, err
	}
	s.conf = sc

	psk := sc.GetPreSharedKey()
	if len(psk) != 32 {
		return nil, errors.New("pre-shared key must be 32 bytes long")
	}
	copy(s.Key[:], psk)

	return s, nil
}

// NewConvergentImmutableStrategy builds a new convergent immutable strategy from scratch.
func NewConvergentImmutableStrategy() (*Strategy, error) {
	strat := &Strategy{}
	if _, err := rand.Read(strat.Key[:]); err != nil {
		return nil, err
	}
	strat.conf = &Config{}
	strat.conf.PreSharedKey = make([]byte, 32)
	copy(strat.conf.PreSharedKey, strat.Key[:])
	return strat, nil
}

// BuildArgs returns the object wrapper for the arguments.
func (s *Strategy) BuildArgs() (*pbobject.ObjectWrapper, []byte, error) {
	return pbobject.NewObjectWrapper(s.conf, pbobject.EncryptionConfig{})
}

// GetEncryptionStrategyType returns the encryption strategy type.
func (s *Strategy) GetEncryptionStrategyType() inca.EncryptionStrategy {
	return StrategyType
}

// GetEncryptionConfig returns the encryption configuration for encrypting an object.
func (s *Strategy) GetEncryptionConfig() pbobject.EncryptionConfig {

	resolver := func(ctx context.Context, blob *objectenc.EncryptedBlob, resourceCtr interface{}) error {
		resource := resourceCtr.(*secretbox.SecretBoxResource)

		if len(resource.Message) == 0 {
			return errors.New("decryption not supported")
		}

		digest, err := sampleLocalDb.DigestData(resource.Message)
		if err != nil {
			return err
		}

		var nonce [24]byte
		copy(nonce[:], digest[len(digest)-24:])
		resource.Key = &s.Key
		resource.Nonce = &nonce
		return nil
	}

	return pbobject.EncryptionConfig{
		EncryptionType: EncType,
		ResourceLookup: resolver,
	}
}

// GetEncryptionConfigWithDigest returns the encryption configuration for a given digest.
func (s *Strategy) GetEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig {
	var nonce [24]byte
	copy(nonce[:], digest[len(digest)-24:])

	resolver := func(ctx context.Context, blob *objectenc.EncryptedBlob, resourceCtr interface{}) error {
		resource := resourceCtr.(*secretbox.SecretBoxResource)
		resource.Key = &s.Key
		resource.Nonce = &nonce
		return nil
	}

	return pbobject.EncryptionConfig{
		EncryptionType: EncType,
		ResourceLookup: resolver,
	}
}

// GetGenesisEncryptionConfig returns the encryption configuration for the genesis block.
func (s *Strategy) GetGenesisEncryptionConfig() pbobject.EncryptionConfig {
	return s.GetEncryptionConfig()
}

// GetGenesisEncryptionConfigWithDigest returns the encryption configuration for the genesis block with a digest.
func (s *Strategy) GetGenesisEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig {
	return s.GetEncryptionConfigWithDigest(digest)
}

// GetNodeMessageEncryptionConfig returns the encryption configuration for the node message.
func (s *Strategy) GetNodeMessageEncryptionConfig() pbobject.EncryptionConfig {
	return s.GetEncryptionConfig()
}

// GetGenesisEncryptionConfigWithDigest returns the encryption configuration for the genesis block with a digest.
func (s *Strategy) GetNodeMessageEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig {
	return s.GetEncryptionConfigWithDigest(digest)
}

// GetBlockEncryptionConfig returns the configuration for block related messages.
func (s *Strategy) GetBlockEncryptionConfig() pbobject.EncryptionConfig {
	return s.GetEncryptionConfig()
}

// GetBlockEncryptionConfigWithDigest returns the block encryption configuration for the genesis block with a digest.
func (s *Strategy) GetBlockEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig {
	return s.GetEncryptionConfigWithDigest(digest)
}

// configTypeID is the type ID of the configuration.
var configTypeID = &pbobject.ObjectTypeID{TypeUuid: "/inca/encryption/convergent-immutable/config"}

// GetObjectTypeID returns the object type string, used to identify types.
func (c *Config) GetObjectTypeID() *pbobject.ObjectTypeID {
	return configTypeID
}

func init() {
	impl.MustRegisterEncryptionStrategyCtor(StrategyType, NewConvergentImmutableStrategyFromConfig)
}
