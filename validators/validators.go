package validators

import (
	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain"
	"github.com/pkg/errors"
)

// builtInValidators contains the sample built-in validators.
var builtInValidators = map[string]func(*chain.Chain) block.Validator{
	"none":  nil,
	"allow": func(*chain.Chain) block.Validator { return &AllowAll{} },
	"deny":  func(*chain.Chain) block.Validator { return &DenyAll{} },
	"immutable-validator-set": NewImmutableValidatorSet,
}

// GetBuiltInValidator returns a built in validator by name.
func GetBuiltInValidator(name string, ch *chain.Chain) (block.Validator, error) {
	v, ok := builtInValidators[name]
	if !ok {
		return nil, errors.Errorf("validator type %s not known", name)
	}

	if v == nil {
		return nil, nil
	}

	return v(ch), nil
}
