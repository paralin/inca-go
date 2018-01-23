package peer

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/encryption/convergentimmutable"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/ipfs"
	"github.com/aperturerobotics/objstore/localdb"
	srdigest "github.com/aperturerobotics/storageref/digest"
	"github.com/aperturerobotics/timestamp"
	api "github.com/ipfs/go-ipfs-api"
	"github.com/libp2p/go-libp2p-crypto"
)

// TestPeer tests the Peer interface.
func TestPeer(t *testing.T) {
	sh := api.NewLocalShell()
	if sh == nil {
		t.Fatal("unable to connect to local ipfs")
	}

	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(logger)
	ctx := context.Background()
	dbm := db.NewInmemDb()
	localStore := localdb.NewLocalDb(db.WithPrefix(dbm, []byte("/localdb")))
	remoteStore := ipfs.NewRemoteStore(sh)
	objStore := objstore.NewObjectStore(ctx, localStore, remoteStore)

	genesisDigest, err := localStore.DigestData([]byte("stub"))
	if err != nil {
		t.Fatal(err.Error())
	}

	nodePriv, nodePub, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		t.Fatal(err.Error())
	}

	encStrat, err := convergentimmutable.NewConvergentImmutableStrategy()
	if err != nil {
		t.Fatal(err.Error())
	}

	p, err := NewPeer(ctx, le, dbm, objStore, nodePub, genesisDigest, encStrat)
	if err != nil {
		t.Fatal(err.Error())
	}

	nowTs := timestamp.Now()
	nodeMessage := &inca.NodeMessage{
		GenesisRef:  srdigest.NewStorageRefDigest(genesisDigest),
		Timestamp:   &nowTs,
		MessageType: inca.NodeMessageType_NodeMessageType_UNKNOWN,
	}
	if err := p.processIncomingNodeMessage(nodeMessage); err != nil {
		t.Fatal(err.Error())
	}
	_ = p
	_ = nodePriv
}
