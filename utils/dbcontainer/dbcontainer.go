package dbcontainer

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/objstore/db"
	"github.com/emirpasic/gods/containers"
)

// Container satisfies the container requirements.
type Container interface {
	containers.Container
	containers.JSONDeserializer
	containers.JSONSerializer
}

// DbContainer wraps a go-datastructures container with serialization to db.
type DbContainer struct {
	Container

	db    db.Db
	dbKey []byte
}

// NewDbContainer builds a new database-backed container.
func NewDbContainer(
	parentDB db.Db,
	containerID string,
	container Container,
) *DbContainer {
	return &DbContainer{
		Container: container,
		db:        parentDB,
		dbKey:     []byte(fmt.Sprintf("/dbcontainer/%s", containerID)),
	}
}

// WriteState writes the container state to the db.
func (d *DbContainer) WriteState(ctx context.Context) error {
	data, err := d.Container.ToJSON()
	if err != nil {
		return err
	}

	return d.db.Set(ctx, d.dbKey, data)
}

// ReadState reads the container state from the db.
func (d *DbContainer) ReadState(ctx context.Context) error {
	data, dataOk, err := d.db.Get(ctx, d.dbKey)
	if err != nil || !dataOk {
		return err
	}

	return d.Container.FromJSON(data)
}
