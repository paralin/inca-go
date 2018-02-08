package convergentimmutable

import (
	"context"
	"crypto/rand"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/encryption/impl"
	ici "github.com/aperturerobotics/inca/encryption/convergentimmutable"
	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/objectenc/secretbox"
	"github.com/aperturerobotics/objstore/localdb"
	"github.com/aperturerobotics/pbobject"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/pkg/errors"
)

// StrategyType is the strategy type of this implementation.
const StrategyType = inca.EncryptionStrategy_EncryptionStrategy_ConvergentImmutable

// EncType is the encryption type of this implementation.
const EncType = objectenc.EncryptionType_EncryptionType_SECRET_BOX

// CmpType is the compression type of this implementation.
const CmpType = objectenc.CompressionType_CompressionType_SNAPPY

// hack: directly call local db compliant digest
var sampleLocalDb = &localdb.LocalDb{}

// Strategy implements the convergent encryption strategy with immutable historic encryption.
// This means that a pre-shared key will be used, along with the digest (sha256) of the object as the nonce.
// If the key is changed in some block, the blocks following the change will be re-encrypted with the new key.
// Immutable indicates that the state cannot be historically re-encrypted if the key is leaked.
type Strategy struct {
	Key  [32]byte
	conf *ici.Config
}

// NewConvergentImmutableStrategyFromConfig builds a new convergent immutable strategy.
func NewConvergentImmutableStrategyFromConfig(conf *pbobject.ObjectWrapper) (encryption.Strategy, error) {
	s := &Strategy{}

	sc := &ici.Config{}
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
	strat.conf = &ici.Config{}
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
		EncryptionType:  EncType,
		ResourceLookup:  resolver,
		CompressionType: CmpType,
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
		EncryptionType:  EncType,
		ResourceLookup:  resolver,
		CompressionType: CmpType,
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
func (s *Strategy) GetNodeMessageEncryptionConfig(nodePriv crypto.PrivKey) pbobject.EncryptionConfig {
	c := s.GetEncryptionConfig()
	c.SignerKeys = append(c.SignerKeys, nodePriv)
	return c
}

// GetNodeMessageEncryptionConfigWithDigest returns the encryption configuration for the genesis block with a digest.
func (s *Strategy) GetNodeMessageEncryptionConfigWithDigest(pubKey crypto.PubKey, digest []byte) pbobject.EncryptionConfig {
	encConf := s.GetEncryptionConfigWithDigest(digest)
	encConf.VerifyKeys = append(encConf.VerifyKeys, pubKey)
	return encConf
}

// GetBlockEncryptionConfig returns the configuration for block related messages.
func (s *Strategy) GetBlockEncryptionConfig() pbobject.EncryptionConfig {
	return s.GetEncryptionConfig()
}

// GetBlockEncryptionConfigWithDigest returns the block encryption configuration for the genesis block with a digest.
func (s *Strategy) GetBlockEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig {
	return s.GetEncryptionConfigWithDigest(digest)
}

func init() {
	impl.MustRegisterEncryptionStrategyCtor(StrategyType, NewConvergentImmutableStrategyFromConfig)
}
