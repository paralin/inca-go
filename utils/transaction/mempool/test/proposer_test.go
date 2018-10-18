package mempool

import (
	"context"
	"crypto/rand"
	"fmt"
	"hash/crc32"
	"testing"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/chain"
	cstate "github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
	"github.com/aperturerobotics/inca-go/utils/transaction/mock"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/aperturerobotics/inca-go/validators"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/objstore/db/inmem"
	"github.com/aperturerobotics/objstore/localdb"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/timestamp"

	"github.com/libp2p/go-libp2p-crypto"
	"github.com/stretchr/testify/require"

	// _ imports all encryption and compression types
	_ "github.com/aperturerobotics/objectenc/all"
	// _ imports all storage ref types
	_ "github.com/aperturerobotics/storageref/all"
)

// TestProposer tests the proposer.
func TestProposer(t *testing.T) {
	// Compute a node private key
	nodePriv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	ctx := context.Background()
	t.Log("constructing inmem db")
	dbm := inmem.NewInmemDb()

	t.Log("constructing txdb")
	txDb, err := txdb.NewTxDatabase(ctx, db.WithPrefix(dbm, []byte("/txdb")))
	require.NoError(t, err)

	t.Log("constructing mempool")
	m, err := mempool.NewMempool(ctx, dbm, txDb, mempool.Opts{
		Orderer: func(ctx context.Context, txDb *txdb.TxDatabase, txID string) (float64, error) {
			h := crc32.NewIEEE()
			_, _ = h.Write([]byte(txID))
			return float64(h.Sum32()), nil
		},
	})
	require.NoError(t, err)

	t.Log("constructing objstore")
	localStore := localdb.NewLocalDb(dbm)
	objStore := objstore.NewObjectStore(ctx, localStore, nil)
	ctx = objstore.WithObjStore(ctx, objStore)

	t.Log("constructing mock app")
	mockApp := &mock.Application{}

	ta := time.Now()
	t.Log("enqueuing 1k transactions")
	for i := 0; i < 1000; i++ {
		txID := fmt.Sprintf("test-%d", i)
		// Place the inner transaction data.
		mockTx := &mock.Transaction{InnerData: []byte(txID)}

		// Store inner transaction data
		txRef, _, err := objStore.StoreObject(ctx, mockTx, pbobject.EncryptionConfig{})
		require.NoError(t, err)

		// Construct node message.
		ts := timestamp.Now()
		txNodeMessage := &inca.NodeMessage{
			MessageType:    inca.NodeMessageType_NodeMessageType_APP,
			InnerRef:       txRef,
			Timestamp:      &ts,
			AppMessageType: transaction.TransactionAppMessageID,
			// GenesisRef: ...,
			// PubKey: ...
		}
		nmRef, _, err := objStore.StoreObject(ctx, txNodeMessage, pbobject.EncryptionConfig{
			SignerKeys: []crypto.PrivKey{nodePriv},
		})
		require.NoError(t, err)

		// Store in txdb
		err = txDb.Put(ctx, txID, nmRef)
		require.NoError(t, err)

		// Store in mempool queue
		err = m.Enqueue(ctx, txID)
		require.NoError(t, err)
	}

	_ = fmt.Println
	_ = timestamp.Now

	tb := time.Now()
	t.Logf("enqueued 1k in %s", tb.Sub(ta).String())

	p, err := mempool.NewProposer(
		ctx,
		m,
		mempool.ProposerOpts{
			MinBlockTransactions: 10,
			// ProposeDeadlineRatio: 0.3,
			ProposeWaitRatio: 0.1,
		},
		mockApp,
	)
	require.NoError(t, err)

	// Construct the initial state.
	var initState mock.AppState
	initAppStateRef, _, err := objStore.StoreObject(ctx, &initState, pbobject.EncryptionConfig{})
	require.NoError(t, err)

	initBlkState := transaction.BlockState{
		ApplicationStateRef: initAppStateRef,
	}
	initBlkStateRef, _, err := objStore.StoreObject(ctx, &initBlkState, pbobject.EncryptionConfig{})
	require.NoError(t, err)

	// Construct the root block
	ch, err := chain.NewChain(
		ctx,
		db.WithPrefix(dbm, []byte("/chain")),
		objStore,
		"mock-chain",
		nodePriv,
		nil,
		initBlkStateRef,
	)
	require.NoError(t, err)

	validator, _ := validators.GetBuiltInValidator("allow", ch)
	ch.SetBlockValidator(validator)
	ch.SetBlockProposer(p)

	headBlock := ch.GetHeadBlock()
	roundStart := time.Now()
	roundEnd := roundStart.Add(time.Second * 10)
	nextBlockRef, err := p.ProposeBlock(
		ch.GetContext(),
		headBlock,
		&cstate.ChainStateSnapshot{
			// Context will be canceled when the state is invalidated.
			Context: ctx,
			// RoundStarted indicates if the round is in progress.
			RoundStarted: true,
			// BlockRoundInfo is the current block height and round information.
			BlockRoundInfo: &inca.BlockRoundInfo{Height: 1, Round: 1},
			// LastBlockHeader is the last block header.
			LastBlockHeader: headBlock.GetHeader(),
			// LastBlockRef is the reference to the last block.
			LastBlockRef: headBlock.GetBlockRef(),
			// RoundEndTime is the time when the round will end.
			// RoundStartTime is the time when the round will/did start.
			RoundStartTime: roundStart,
			// NextRoundStartTime is the time when the next round will start.
			NextRoundStartTime: roundEnd.Add(time.Second),
		},
	)
	require.NoError(t, err)
	require.NotNil(t, nextBlockRef)

	mockState := mock.AppState{}
	err = nextBlockRef.FollowRef(ctx, nil, &mockState, nil)
	require.NoError(t, err)

	if mockState.GetAppliedTxCount() != 1 {
		t.Fatalf("expected 1 txcount, actual: %d", mockState.GetAppliedTxCount())
	}
}
