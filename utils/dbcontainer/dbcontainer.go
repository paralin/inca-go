package dbcontainer

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/hydra/object"
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

	store object.ObjectStore
	dbKey string
}

// NewDbContainer builds a new database-backed container.
func NewDbContainer(
	parentDB object.ObjectStore,
	containerID string,
	container Container,
) *DbContainer {
	return &DbContainer{
		Container: container,
		store:     parentDB,
		dbKey:     fmt.Sprintf("dbcontainer/%s", containerID),
	}
}

// WriteState writes the container state to the db.
func (d *DbContainer) WriteState(ctx context.Context) error {
	data, err := d.Container.ToJSON()
	if err != nil {
		return err
	}

	return d.store.SetObject(d.dbKey, data)
}

// ReadState reads the container state from the db.
func (d *DbContainer) ReadState(ctx context.Context) error {
	data, dataOk, err := d.store.GetObject(d.dbKey)
	if err != nil || !dataOk {
		return err
	}

	return d.Container.FromJSON(data)
}
