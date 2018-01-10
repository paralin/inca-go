package db

import (
	"context"
	"path"

	"github.com/aperturerobotics/pbobject"
)

// dbPrefixer prefixes everything going in and out of a db.
type dbPrefixer struct {
	db     Db
	prefix string
}

// applyPrefix applies the prefix to a key.
func (d *dbPrefixer) applyPrefix(key string) string {
	return path.Join(d.prefix, key)
}

// Get retrieves an object from the database.
// Not found should return nil, nil
func (d *dbPrefixer) Get(ctx context.Context, key string) (pbobject.Object, error) {
	return d.db.Get(ctx, d.applyPrefix(key))
}

// Set sets an object in the database.
func (d *dbPrefixer) Set(ctx context.Context, key string, val pbobject.Object) error {
	return d.db.Set(ctx, d.applyPrefix(key), val)
}

// WithPrefix adds a prefix to a database.
func WithPrefix(d Db, prefix string) Db {
	return &dbPrefixer{db: d, prefix: prefix}
}
