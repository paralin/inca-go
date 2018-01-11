package node

import (
	"github.com/libp2p/go-libp2p-crypto"
)

// UnmarshalPrivKey unmarshals the private key.
func (c *Config) UnmarshalPrivKey() (crypto.PrivKey, error) {
	return crypto.UnmarshalPrivateKey(c.PrivKey)
}

// SetPrivKey sets the private key.
func (c *Config) SetPrivKey(priv crypto.PrivKey) error {
	privDat, err := crypto.MarshalPrivateKey(priv)
	if err != nil {
		return err
	}

	c.PrivKey = privDat
	return nil
}
