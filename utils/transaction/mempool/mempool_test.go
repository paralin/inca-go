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
	t.Log("constructing inmem db")
	dbm := inmem.NewInmemDb()

	t.Log("constructing txdb")
	txDb, err := txdb.NewTxDatabase(ctx, db.WithPrefix(dbm, []byte("/txdb")))
	require.NoError(t, err)

	t.Log("constructing mempool")
	m, err := NewMempool(ctx, dbm, txDb, Opts{
		Orderer: func(ctx context.Context, txDb *txdb.TxDatabase, txID string) (float64, error) {
			h := crc32.NewIEEE()
			_, _ = h.Write([]byte(txID))
			return float64(h.Sum32()), nil
		},
	})
	require.NoError(t, err)

	ta := time.Now()
	t.Log("enqueuing 1k transactions")
	var err error
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
	errCh := make(chan error, 1)
	go func() {
		err := m.CollectTransactions(ctx, 100, outCh)
		if err != nil {
			errCh <- err
		} else {
			close(outCh)
		}
	}()

	outI := 0
	te := time.Now()
OuterLoop:
	for {
		select {
		case err := <-errCh:
			t.Fatal(err.Error())
		case _, ok := <-outCh:
			if !ok {
				break OuterLoop
			}
			outI++
		}
	}

	require.Equal(t, 100, outI)
	t.Logf("collected min 3-103 in %s", time.Now().Sub(te).String())
}
