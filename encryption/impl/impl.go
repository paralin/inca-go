package impl

import (
	"sync"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/pbobject"
	"github.com/pkg/errors"
)

// EncryptionStrategyCtor constructs an encryption strategy with a optional configuration.
type EncryptionStrategyCtor = func(conf *pbobject.ObjectWrapper) (encryption.Strategy, error)

// ErrDuplicateImpl is returned when a duplicate encryption implementation is registered.
var ErrDuplicateImpl = errors.New("duplicate encryption implementation")

// encryptionImplCtorsMtx is the mutex on the encryptionImplCtors map
var encryptionImplCtorsMtx sync.RWMutex

// encryptionImplCtors contains registered implementations.
var encryptionImplCtors = make(map[inca.EncryptionStrategy]EncryptionStrategyCtor)

// MustRegisterEncryptionStrategyCtor registers an encryption implementation constructor or panics.
// expected to be called from Init(), but can be deferred
func MustRegisterEncryptionStrategyCtor(stratType inca.EncryptionStrategy, impl EncryptionStrategyCtor) {
	if err := RegisterEncryptionStrategyCtor(stratType, impl); err != nil {
		panic(err)
	}
}

// RegisterEncryptionStrategyCtor registers an encryption implementation.
// expected to be called from Init(), but can be deferred
func RegisterEncryptionStrategyCtor(encType inca.EncryptionStrategy, impl EncryptionStrategyCtor) error {
	encryptionImplCtorsMtx.Lock()
	defer encryptionImplCtorsMtx.Unlock()

	if _, ok := encryptionImplCtors[encType]; ok {
		return ErrDuplicateImpl
	}

	encryptionImplCtors[encType] = impl
	return nil
}

// GetEncryptionStrategyCtor returns the registered implementation constructor of the type.
func GetEncryptionStrategyCtor(kind inca.EncryptionStrategy) (impl EncryptionStrategyCtor, err error) {
	if _, ok := inca.EncryptionStrategy_name[int32(kind)]; !ok {
		return nil, errors.Errorf("encryption strategy type unknown: %v", kind.String())
	}

	encryptionImplCtorsMtx.RLock()
	impl = encryptionImplCtors[kind]
	encryptionImplCtorsMtx.RUnlock()

	if impl == nil {
		err = errors.Errorf("unimplemented encryption strategy type: %v", kind.String())
	}
	return
}

// BuildEncryptionStrategy tries to build an encryption strategy given type and args.
func BuildEncryptionStrategy(kind inca.EncryptionStrategy, args *pbobject.ObjectWrapper) (encryption.Strategy, error) {
	ctor, err := GetEncryptionStrategyCtor(kind)
	if err != nil {
		return nil, err
	}

	return ctor(args)
}
