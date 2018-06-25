package mempool

import (
	"context"
	"fmt"
	"hash/crc32"
	"testing"
	"time"

	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/objstore/db/inmem"
	// "github.com/stretchr/testify/assert"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/stretchr/testify/require"
)

// TestMempool tests the mempool.
func TestMempool(t *testing.T) {
	ctx := context.Background()
	dbm := inmem.NewInmemDb()

	txDb, err := txdb.NewTxDatabase(ctx, db.WithPrefix(dbm, []byte("/txdb")))
	require.NoError(t, err)

	m, err := NewMempool(ctx, dbm, txDb, Opts{
		Orderer: func(ctx context.Context, txDb *txdb.TxDatabase, txID string) (float64, error) {
			h := crc32.NewIEEE()
			_, _ = h.Write([]byte(txID))
			return float64(h.Sum32()), nil
		},
	})
	require.NoError(t, err)

	ta := time.Now()
	for i := 0; i < 1000; i++ {
		err = m.Enqueue(ctx, fmt.Sprintf("test-%d", i))
		require.NoError(t, err)
	}
	tb := time.Now()
	t.Logf("enqueued 1k in %s", tb.Sub(ta).String())

	txID, err := m.DequeueMin(ctx)
	require.NoError(t, err)
	require.Equal(t, "test-607", txID)

	tc := time.Now()
	t.Logf("dequeued min 1 in %s", tc.Sub(tb).String())

	txID, err = m.DequeueMin(ctx)
	require.NoError(t, err)
	td := time.Now()
	t.Logf("dequeued min 2 in %s", td.Sub(tc).String())

	require.Equal(t, "test-203", txID)

	outCh := make(chan string, 100)
	err = m.CollectTransactions(ctx, 100, outCh)
	require.NoError(t, err)

	outI := 0
	te := time.Now()
	for _ = range outCh {
		outI++
	}
	require.Equal(t, 100, outI)
	t.Logf("collected min 3-103 in %s", time.Now().Sub(te).String())
}
