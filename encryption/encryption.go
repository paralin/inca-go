package encryption

import (
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/pbobject"
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
	GetNodeMessageEncryptionConfig() pbobject.EncryptionConfig
	// GetNodeMessageEncryptionConfigWithDigest returns the encryption configuration for the node message with a digest.
	GetNodeMessageEncryptionConfigWithDigest(digest []byte) pbobject.EncryptionConfig
}
