package mempool

import (
	"context"

	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/objstore/dbds/fibheap"
)

// Mempool is an implementation of an In-Memory Queued Transaction Pool.
// The state, and transactions against the state, are opaque interfaces to this module.
// Transactions are sent over the Inca NodeMessage channel, with any opaque storage reference'd transaction body.
// Transaction submission time and author are inferred from the containing NodeMessage.
type Mempool struct {
	opts   Opts
	txHeap *fibheap.FibbonaciHeap
}

// Opts are options passed to the mempool to tweak functionality.
type Opts struct {
}

// NewMempool constructs a mempool, loading state from the database.
func NewMempool(
	ctx context.Context,
	dbm db.Db,
	mempoolOpts Opts,
) (m *Mempool, err error) {
	m = &Mempool{opts: mempoolOpts}
	m.txHeap, err = fibheap.NewFibbonaciHeap(
		ctx,
		db.WithPrefix(dbm, []byte("/txpool")),
	)
	if err != nil {
		return nil, err
	}

	return m, nil
}

// CollectTransactions collects transactions from the mempool.
// Transactions are collected until the context is canceled.
func (m *Mempool) CollectTransactions(ctx context.Context) {
}
