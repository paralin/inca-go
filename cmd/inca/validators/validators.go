package validators

import (
	"github.com/aperturerobotics/inca-go/block"
	"github.com/pkg/errors"
)

// BuiltInValidator satisfies all the validator interfaces.
type BuiltInValidator interface {
	block.Validator
}

// builtInValidators contains the sample built-in validators.
var builtInValidators = map[string]BuiltInValidator{
	"none":  nil,
	"allow": &AllowValidator{},
	"deny":  &DenyValidator{},
}

// GetBuiltInValidator returns a built in validator by name.
func GetBuiltInValidator(name string) (BuiltInValidator, error) {
	v, ok := builtInValidators[name]
	if !ok {
		return nil, errors.Errorf("validator type %s not known", name)
	}

	return v, nil
}
