package objtable

import (
	"context"

	"github.com/aperturerobotics/objectenc"
	"github.com/aperturerobotics/objectenc/resources"
	"github.com/aperturerobotics/pbobject"
)

// ObjectTable is the Inca object table.
type ObjectTable struct {
	*pbobject.ObjectTable
	resourceResolver ResourceResolver
}

// ResourceResolver resolves Inca resources.
type ResourceResolver interface {
}

// NewObjectTable builds a new ObjectTable.
func NewObjectTable() *ObjectTable {
	table := pbobject.NewObjectTable()
	return Wrap(table)
}

// Wrap builds a ObjectTable from an existing pbobject table.
func Wrap(table *pbobject.ObjectTable) *ObjectTable {
	t := &ObjectTable{ObjectTable: table}
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
		// mh is the key salt multihash to look up.
		mh := resource.KeySaltMultihash
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
