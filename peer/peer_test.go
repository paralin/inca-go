package peer

import (
	"context"
	"crypto/rand"
	"testing"

	"github.com/aperturerobotics/controllerbus/config"
	"github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/block/transform"
	"github.com/aperturerobotics/hydra/block/transform/chksum"
	"github.com/aperturerobotics/hydra/block/transform/snappy"
	"github.com/aperturerobotics/hydra/testbed"
	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/timestamp"
	"github.com/libp2p/go-libp2p-crypto"
	"github.com/sirupsen/logrus"
)

// TestPeer tests the Peer interface.
func TestPeer(t *testing.T) {
	ctx := context.Background()
	log := logrus.New()
	log.SetLevel(logrus.DebugLevel)
	le := logrus.NewEntry(log)

	tb, err := testbed.NewTestbed(ctx, le)
	if err != nil {
		t.Fatal(err.Error())
	}

	vol := tb.Volume
	volID := vol.GetID()
	t.Log(volID)

	// construct a basic transform config.
	tconf, err := block_transform.NewConfig([]config.Config{
		&transform_chksum.Config{},
		&transform_snappy.Config{},
	})
	if err != nil {
		t.Fatal(err.Error())
	}

	oc, _, err := object.BuildEmptyCursor(
		ctx,
		tb.Bus,
		tb.Logger,
		tb.StepFactorySet,
		testbed.BucketId,
		volID,
		tconf,
		nil,
	)
	if err != nil {
		t.Fatal(err.Error())
	}

	validatorPriv, validatorPub, err := crypto.GenerateEd25519Key(rand.Reader)
	_ = validatorPriv
	if err != nil {
		t.Fatal(err.Error())
	}

	store, err := tb.Volume.OpenObjectStore(ctx, "test-store")
	if err != nil {
		t.Fatal(err.Error())
	}

	// store dummy genesis object
	genTx, genCursor := oc.BuildTransaction(nil)
	genCursor.SetBlock(&inca.Genesis{ChainId: "test"})
	eves, genCursor, err := genTx.Write()
	if err != nil {
		t.Fatal(err.Error())
	}
	genesisRef := eves[len(eves)-1].GetPutBlock().GetBlockCommon().GetBlockRef()

	p, err := NewPeer(ctx, le, oc, store, validatorPub, genesisRef, nil)
	if err != nil {
		t.Fatal(err.Error())
	}

	np, _ := validatorPub.Bytes()
	nowTs := timestamp.Now()
	nodeMessage := &inca.NodeMessage{
		GenesisRef:  genesisRef,
		Timestamp:   &nowTs,
		MessageType: inca.NodeMessageType_NodeMessageType_UNKNOWN,
		PubKey:      np,
	}
	genCursor.SetBlock(nodeMessage)
	eves, genCursor, err = genTx.Write()
	if err != nil {
		t.Fatal(err.Error())
	}
	nmRef := eves[len(eves)-1].GetPutBlock().GetBlockCommon().GetBlockRef()
	if err := p.processIncomingNodeMessage(nodeMessage, nmRef); err != nil {
		t.Fatal(err.Error())
	}
	_ = nmRef
}
