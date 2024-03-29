package chain

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/node"
	"github.com/aperturerobotics/inca-go/validators"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/objstore/db/inmem"
	"github.com/aperturerobotics/objstore/ipfs"
	"github.com/aperturerobotics/objstore/localdb"
	api "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p-crypto"
)

// TestChain tests the chain methods.
func TestChain(t *testing.T) {
	sh := api.NewLocalShell()
	if sh == nil {
		t.Fatal("unable to connect to local ipfs")
	}

	ctx := context.Background()
	dbm := inmem.NewInmemDb()
	localStore := localdb.NewLocalDb(db.WithPrefix(dbm, []byte("/localdb")))
	remoteStore := ipfs.NewRemoteStore(sh)
	objStore := objstore.NewObjectStore(ctx, localStore, remoteStore)

	validatorPriv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}

	nod, err := node.NewNode(ctx, db.WithPrefix(dbm, "/node"), objStore, sh, ch, validatorPriv)
	if err != nil {
		t.Fatal(err.Error())
	}

	chainID := "test-chain-1"
	ch, err := NewChain(ctx, dbm, objStore, chainID, validatorPriv, &validators.AllowAll{})
	if err != nil {
		t.Fatal(err.Error())
	}

	{
		chAfter, err := FromConfig(ctx, dbm, objStore, ch.GetConfig())
		if err != nil {
			t.Fatal(err.Error())
		}

		if chAfter.GetPubsubTopic() != ch.GetPubsubTopic() {
			t.Fail()
		}
		ch = chAfter
	}

	proposer, err := NewProposer(ctx, validatorPriv, dbm, ch, nod)
	if err != nil {
		t.Fatal(err.Error())
	}
	_ = proposer
}
