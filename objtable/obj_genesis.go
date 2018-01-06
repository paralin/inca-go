package objtable

import (
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/pbobject"

	// import the AES algorithms
	_ "github.com/aperturerobotics/objectenc/aes"
)

func init() {
	registerObjectCtor(func() pbobject.Object { return &inca.Genesis{} })
	// set the default write-time encryption
	registerObjectEncryption(&inca.Genesis{}, pbobject.EncryptionConfig{
		EncryptionType: objectenc.EncryptionType_EncryptionType_AES,
	})
}
