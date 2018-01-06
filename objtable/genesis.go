package objtable

import (
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/pbobject"
)

// GenesisObject wraps Genesis to make it a pbobject.
type GenesisObject struct {
	*inca.Genesis
}

// NewGenesisObject builds a new GenesisObject.
func NewGenesisObject(g *inca.Genesis) *GenesisObject {
	return &GenesisObject{Genesis: g}
}

// GetObjectTypeID returns the object type string, used to identify types.
func (o *GenesisObject) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/genesis/0.0.1")
}

func init() {
	registerObjectCtor(func() pbobject.Object { return NewGenesisObject(&inca.Genesis{}) })
	// set the default write-time encryption
	registerObjectEncryption(&GenesisObject{}, pbobject.EncryptionConfig{
		EncryptionType: objectenc.EncryptionType_EncryptionType_AES,
	})
}
