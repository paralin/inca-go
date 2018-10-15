package mempool

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/objstore/dbds/fibheap"
	"github.com/aperturerobotics/pbobject"
	"github.com/pkg/errors"
)

// Mempool is an implementation of an In-Memory Queued Transaction Pool.
// The state, and transactions against the state, are opaque interfaces to this module.
// Transactions are sent over the Inca NodeMessage channel, with any opaque storage reference'd transaction body.
// Transaction submission time and author are inferred from the containing NodeMessage.
type Mempool struct {
	ctx           context.Context
	txHeap        *fibheap.FibbonaciHeap
	txDb          *txdb.TxDatabase
	orderer       Orderer
	enqueueNotify sync.Map // map[uint32]func()
	notifyIDCtr   uint32
}

// Orderer determines the priority value for a transaction.
type Orderer func(ctx context.Context, txDb *txdb.TxDatabase, txID string) (float64, error)

// TimestampOrderer orders transaction by timestamp.
func TimestampOrderer(ctx context.Context, txDb *txdb.TxDatabase, txID string) (float64, error) {
	txInfo, err := txDb.Get(ctx, txID)
	if err != nil {
		return 0, err
	}

	if txInfo == nil {
		return 0, errors.Errorf("transaction not found: %s", txID)
	}

	nodeMessage := &inca.NodeMessage{}
	nodeMessageWrapper := &pbobject.ObjectWrapper{}
	if err := txInfo.GetNodeMessageRef().FollowRef(
		ctx,
		nil,
		nodeMessage,
		nodeMessageWrapper,
	); err != nil {
		return 0, err
	}

	return float64(nodeMessage.GetTimestamp().GetTimeUnixMs()), nil
}

// Opts are options passed to the mempool to tweak functionality.
type Opts struct {
	// Orderer controls the order transactions are processed.
	// Default is earliest-first.
	Orderer Orderer
}

// NewMempool constructs a mempool, loading state from the database.
// Ctx is used as the root context for database operations.
func NewMempool(
	ctx context.Context,
	dbm db.Db,
	txDb *txdb.TxDatabase,
	mempoolOpts Opts,
) (m *Mempool, err error) {
	m = &Mempool{txDb: txDb, ctx: ctx}
	m.txHeap, err = fibheap.NewFibbonaciHeap(
		ctx,
		db.WithPrefix(dbm, []byte("/txheap")),
	)
	if err != nil {
		return nil, err
	}

	orderer := mempoolOpts.Orderer
	if orderer == nil {
		orderer = TimestampOrderer
	}

	m.orderer = orderer
	return m, nil
}

// Enqueue adds a transaction from the database into the pool.
func (m *Mempool) Enqueue(ctx context.Context, txID string) error {
	prio, err := m.orderer(ctx, m.txDb, txID)
	if err != nil {
		return err
	}

	return m.txHeap.Enqueue(m.ctx, txID, prio)
}

// DequeueMin removes the next transaction from the pool.
func (m *Mempool) DequeueMin(ctx context.Context) (txID string, rerr error) {
	txID, _, rerr = m.txHeap.DequeueMin(m.ctx)
	return
}

// CollectTransactions collects transactions while ctx is active.
// If max > 0, when max tx are reached the function will return.
func (m *Mempool) CollectTransactions(
	ctx context.Context,
	max int,
	outCh chan<- string,
) error {
	enqueueNot := make(chan struct{}, 1)
	enqueueID := atomic.AddUint32(&m.notifyIDCtr, 1)
	m.enqueueNotify.Store(enqueueID, func() {
		select {
		case enqueueNot <- struct{}{}:
		default:
		}
	})
	defer close(outCh)
	defer m.enqueueNotify.Delete(enqueueID)

	for {
		txID, err := m.DequeueMin(m.ctx)
		if err != nil {
			return err
		}

		if txID != "" {
			select {
			case <-ctx.Done():
				_ = m.Enqueue(m.ctx, txID)
				return ctx.Err()
			case outCh <- txID:
			}

			if max != 0 {
				max--
				if max == 0 {
					return nil
				}
			}

			continue
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-enqueueNot:
		}
	}
}

// GetTransactionDb returns the transaction database.
func (m *Mempool) GetTransactionDb() *txdb.TxDatabase {
	return m.txDb
}
