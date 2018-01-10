package db

import (
	"context"

	"github.com/aperturerobotics/pbobject"
)

// Db is an implementation of the Inca database.
type Db interface {
	// Get retrieves an object from the database.
	// Not found should return nil, nil
	Get(ctx context.Context, key string) (pbobject.Object, error)
	// Set sets an object in the database.
	Set(ctx context.Context, key string, val pbobject.Object) error
}
