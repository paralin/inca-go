package main

import (
	"sync"

	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/ipfs"
	"github.com/aperturerobotics/objstore/localdb"
)

var objStoreMtx sync.Mutex
var objStoreCached *objstore.ObjectStore

func GetObjectStore() (*objstore.ObjectStore, error) {
	objStoreMtx.Lock()
	defer objStoreMtx.Unlock()

	if objStoreCached != nil {
		return objStoreCached, nil
	}

	db, err := GetDb()
	if err != nil {
		return nil, err
	}

	sh, err := GetShell()
	if err != nil {
		return nil, err
	}

	localStore := localdb.NewLocalDb(db)
	remoteStore := ipfs.NewRemoteStore(sh.Shell)
	objStore := objstore.NewObjectStore(rootContext, localStore, remoteStore)
	objStoreCached = objStore
	return objStoreCached, nil
}
