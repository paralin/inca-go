package objtable

import (
	"context"
	"encoding/base64"

	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/objectenc/resources"
	"github.com/aperturerobotics/objectenc/resources/keystore"
	"github.com/aperturerobotics/pbobject"
	"github.com/pkg/errors"
)

var testBase64Key = "EXat7HD9lYyGpfzkmzNmkM0ij6n2+MSSjFqlReRPaOE="

func getTestKey() []byte {
	key, err := base64.StdEncoding.DecodeString(testBase64Key)
	if err != nil {
		panic(err)
	}
	return key
}

// ObjectTable is the Inca object table.
type ObjectTable struct {
	*pbobject.ObjectTable
	*keystore.KeyResourceStore
	resourceResolver ResourceResolver
}

// ResourceResolver resolves Inca resources.
type ResourceResolver interface {
}

// NewObjectTable builds a new ObjectTable.
func NewObjectTable() *ObjectTable {
	table := pbobject.NewObjectTable()
	ks := keystore.NewKeyResourceStore()
	ks.AddKey(getTestKey())
	return Wrap(table, ks)
}

// Wrap builds a ObjectTable from an existing pbobject table.
func Wrap(table *pbobject.ObjectTable, keyStore *keystore.KeyResourceStore) *ObjectTable {
	t := &ObjectTable{ObjectTable: table, KeyResourceStore: keyStore}
	t.registerIncaTypes()
	return t
}

// SetResourceResolver sets the table resource resolver.
func (o *ObjectTable) SetResourceResolver(res ResourceResolver) {
	o.resourceResolver = res
}

// resolveEncryptionResource resolves an encryption resource.
// Satisfies objectenc.ResourceResolverFunc
func (o *ObjectTable) resolveEncryptionResource(ctx context.Context, blob *objectenc.EncryptedBlob, resourceCtr interface{}) error {
	switch resource := resourceCtr.(type) {
	case *resources.KeyResource:
		if resource.Encrypting {
			resource.KeyData = getTestKey()
			return nil
		}

		key, ok := o.KeyResourceStore.GetByMultihash(resource.KeySaltMultihash)
		if !ok {
			return errors.Errorf("unknown %s key: %s", resource.EncryptionType, resource.KeySaltMultihash.GetKeyMultihash().B58String())
		}
		resource.KeyData = key
	default:
		return errors.Errorf("unhandled resource: %#v", resource)
	}
	return nil
}

// objectCtors contains the registered object constructors.
var objectCtors []func() pbobject.Object

// registerObjectCtor adds an object constructor at init time.
func registerObjectCtor(ctor func() pbobject.Object) {
	objectCtors = append(objectCtors, ctor)
}

// objectEncryption contains the registered object encryption configs.
var objectEncryption = make(map[uint32]pbobject.EncryptionConfig)

// registerObjectEncryption adds a default object encryption config at init time.
func registerObjectEncryption(obj pbobject.Object, encConfig pbobject.EncryptionConfig) {
	typeID := obj.GetObjectTypeID().GetCrc32()
	objectEncryption[typeID] = encConfig
}

// registerIncaTypes registers the inca types into a table.
func (o *ObjectTable) registerIncaTypes() {
	t := o.ObjectTable
	for _, ctor := range objectCtors {
		if err := t.RegisterType(false, ctor); err != nil {
			// cannot happen with overwrite = false
			panic(err)
		}
	}
	for id, enc := range objectEncryption {
		enc.ResourceLookup = o.resolveEncryptionResource
		t.RegisterTypeEncryption(id, enc)
	}
}
