package ipfs

import (
	"bytes"
	"context"
	"io/ioutil"

	"github.com/aperturerobotics/inca-go/objstore"
	api "github.com/ipfs/go-ipfs-api"
)

// RemoteStore implements objstore.RemoteStore
type RemoteStore struct {
	shell *api.Shell
}

// NewRemoteStore builds a new remote store.
func NewRemoteStore(sh *api.Shell) *RemoteStore {
	return &RemoteStore{shell: sh}
}

// FetchRemote returns a blob from blob storage given the storage reference string.
func (r *RemoteStore) FetchRemote(ctx context.Context, storageRef string) ([]byte, error) {
	rc, err := r.shell.Cat(storageRef)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	return ioutil.ReadAll(rc)
}

// StoreRemote stores a blob in blob storage and returns the storage reference string.
func (r *RemoteStore) StoreRemote(ctx context.Context, blob []byte) (string, error) {
	return r.shell.Add(bytes.NewReader(blob))
}

// _ is a type assertion
var _ objstore.RemoteStore = &RemoteStore{}
