package encryption

import (
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/pbobject"
	"github.com/libp2p/go-libp2p-crypto"
)

// Strategy is a blockchain encryption implementation strategy.
// TODO:
//  - Apply mutation to state to push a new block header onto the stack
type Strategy interface {
	// GetEncryptionStrategyType returns the encryption strategy type.
	GetEncryptionStrategyType() inca.EncryptionStrategy
	// GetGenesisEncryptionConfig returns the encryption configuration for encrypting the genesis block.
	GetGenesisEncryptionConfig() pbobject.EncryptionConfig
	// GetGenesisEncryptionConfigWithDigest returns the encryption configuration for the genesis block with a digest.
	GetGenesisEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig
	// GetNodeMessageEncryptionConfig returns the encryption configuration for encrypting a node message.
	GetNodeMessageEncryptionConfig(privKey crypto.PrivKey) pbobject.EncryptionConfig
	// GetNodeMessageEncryptionConfigWithDigest returns the encryption configuration for the node message with a digest.
	GetNodeMessageEncryptionConfigWithDigest(pubKey crypto.PubKey, digest []byte) pbobject.EncryptionConfig
	// GetBlockEncryptionConfig returns the encryption configuration for block messages.
	GetBlockEncryptionConfig() pbobject.EncryptionConfig
	// GetNodeMessageEncryptionConfigWithDigest returns the encryption configuration for the node message with a digest.
	GetBlockEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig
}
