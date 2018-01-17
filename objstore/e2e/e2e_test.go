package e2e

import (
	"context"
	"testing"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/objstore"
	"github.com/aperturerobotics/inca-go/objstore/ipfs"
	"github.com/aperturerobotics/inca-go/objstore/localdb"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/timestamp"

	api "github.com/ipfs/go-ipfs-api"
)

// TestGetOrFetch tests getting or fetching an object from IPFS storage.
func TestGetOrFetch(t *testing.T) {
	ctx := context.Background()
	sh := api.NewLocalShell()
	if sh == nil {
		t.Fatal("unable to connect to local ipfs")
	}

	localStore := localdb.NewLocalDb(db.NewInmemDb())
	remoteStore := ipfs.NewRemoteStore(sh)
	objStore := objstore.NewObjectStore(ctx, localStore, remoteStore)

	genesisTs := timestamp.Now()
	genesis := &inca.Genesis{
		ChainId:   "test-chain",
		Timestamp: &genesisTs,
	}

	encConf := pbobject.EncryptionConfig{Context: ctx}
	storageRef, objData, err := objStore.StoreObject(ctx, genesis, encConf)
	if err != nil {
		t.Fatal(err.Error())
	}

	t.Logf("storage reference: %s", storageRef)
	defer sh.Unpin(storageRef)

	digest, err := localStore.DigestData(objData)
	if err != nil {
		t.Fatal(err.Error())
	}

	outp := &inca.Genesis{}
	if err := objStore.GetOrFetch(ctx, digest, storageRef, outp, encConf); err != nil {
		t.Fatal(err.Error())
	}

	t.Logf("retrieved from storage: %s", outp.String())
}
