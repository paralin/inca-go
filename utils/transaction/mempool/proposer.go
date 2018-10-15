package mempool

import (
	"context"
	"sync"
	"time"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/chain/state"
	"github.com/aperturerobotics/inca-go/logctx"
	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
	"github.com/pkg/errors"
)

// Proposer is a block proposer that pulls transactions from a mempool.
type Proposer struct {
	opts     ProposerOpts
	mempool  *Mempool
	app      Application
	appState ApplicationState
}

// NewProposer builds a new proposer.
func NewProposer(
	memPool *Mempool,
	opts ProposerOpts,
	app Application,
) (*Proposer, error) {
	return &Proposer{
		opts:     opts,
		mempool:  memPool,
		app:      app,
		appState: app.GetState(),
	}, nil
}

// ProposerOpts are options passed to the mempool proposer.
type ProposerOpts struct {
	// ProposeDeadlineRatio is the max percentage of round duration to use for gathering transactions.
	// After this deadline, if at least one transaction was found, it will be processed in the block.
	// If EmitEmptyBlocks is set, this timing will also be used to emit the empty block.
	// If this is set to 1, the proposal will be fired after the wait ratio.
	// If this is set to 0, a default of 0.5 is used.
	ProposeDeadlineRatio float32
	// ProposeWaitRatio is the min percentage of round duration to use for gathering transactions.
	// No proposal will be emitted before this time. This is to throttle block emission rate.
	// If this is set to 0, the proposal will be fired after a single tx at minimum is found.
	// If this is set to 1, a default of 0.15 is used.
	ProposeWaitRatio float32
	// MinBlockTransactions is the minimum number of transactions to have in a block.
	// Failure to collect this many transactions will cause the proposer to abstain.
	MinBlockTransactions int32
	// MaxBlockTransactions is the max number of transactions per block.
	// Reaching this number of transactions will immediately emit the block.
	MaxBlockTranasctions int32
	// EncryptionConfig sets the encryption config to use.
	EncryptionConfig pbobject.EncryptionConfig
}

// Validate validates the data.
func (o *ProposerOpts) Validate() error {
	if o.ProposeDeadlineRatio > 1 || o.ProposeDeadlineRatio < 0 {
		return errors.Errorf("propose deadline ratio out of range (0, 1): %v", o.ProposeDeadlineRatio)
	}

	if o.ProposeWaitRatio > 1 || o.ProposeWaitRatio < 0 {
		return errors.Errorf("propose wait ratio out of range (0, 1): %v", o.ProposeWaitRatio)
	}

	return nil
}

// ProposeBlock proposes a state for the next block or returns none to abstain.
func (p *Proposer) ProposeBlock(
	ctx context.Context,
	parentBlk *block.Block,
	chainState *state.ChainStateSnapshot,
) (*storageref.StorageRef, error) {
	var collectCtx context.Context
	var collectCtxCancel context.CancelFunc

	roundDur := chainState.RoundEndTime.Sub(chainState.RoundStartTime)
	waitUntil := chainState.RoundStartTime.Add(time.Duration(p.opts.ProposeWaitRatio) * roundDur)
	if deadlineRatio := p.opts.ProposeDeadlineRatio; deadlineRatio != 0 {
		deadlineTime := chainState.RoundStartTime.Add(roundDur * time.Duration(deadlineRatio))
		collectCtx, collectCtxCancel = context.WithDeadline(ctx, deadlineTime)
	} else {
		collectCtx, collectCtxCancel = context.WithCancel(ctx)
	}
	defer collectCtxCancel()

	// Stage: collect transactions
	collectedTxIds := make(chan string, 5)
	errCh := make(chan error, 3)
	go func() {
		errCh <- p.mempool.CollectTransactions(
			collectCtx,
			int(p.opts.MaxBlockTranasctions),
			collectedTxIds,
		)
	}()

	// Stage: resolve transactions
	resolvedTxs := make(chan *transaction.Transaction, 5)
	beforeState := p.appState
	afterState := beforeState.Snapshot()
	go func() {
		defer close(resolvedTxs)
		for txID := range collectedTxIds {
			select {
			case <-collectCtx.Done():
				_ = p.mempool.Enqueue(ctx, txID)
				continue
			default:
			}

			txDb := p.mempool.GetTransactionDb()
			tx, err := txDb.GetTx(collectCtx, txID)
			if err != nil {
				if err == context.Canceled || err == context.DeadlineExceeded {
					_ = p.mempool.Enqueue(ctx, txID)
					continue
				}

				continue // drop this tx
			}

			sysErr, txErr := beforeState.PrePoolCheckTx(ctx, tx)
			if txErr != nil {
				if sysErr {
					errCh <- txErr
					_ = p.mempool.Enqueue(ctx, txID)
					collectCtxCancel()
				}

				// skip this tx
				continue
			}

			select {
			case <-collectCtx.Done():
			case resolvedTxs <- tx:
			}
		}
	}()

	// Pay attention to timing and cancel collect ctx.
	var txSetMtx sync.Mutex
	txSet := &transaction.TransactionSet{}
	txProcessedNotify := make(chan struct{}, 1)
	go func() {
		// Recheck every 500ms and/or when tx was processed
		recheckTicker := time.NewTicker(time.Millisecond * 500)
		defer recheckTicker.Stop()
		for {
			now := time.Now()
			afterWaitUntil := waitUntil.Before(now)

			minTx := p.opts.MinBlockTransactions
			txSetMtx.Lock()
			txRefs := txSet.TransactionRefs
			txSetMtx.Unlock()
			txCount := len(txRefs)
			if minTx > 0 && afterWaitUntil && txCount >= int(minTx) {
				collectCtxCancel()
				return
			}

			select {
			case <-collectCtx.Done():
				return
			case <-txProcessedNotify:
			case <-recheckTicker.C:
			}
		}
	}()

	// Stage: apply transactions
	var appliedTxs []string
	for tx := range resolvedTxs {
		appliedTxs = append(appliedTxs, tx.GetID())
		sysErr, txErr := afterState.Apply(ctx, tx)
		if txErr != nil {
			if sysErr {
				errCh <- txErr
				_ = p.mempool.Enqueue(ctx, tx.GetID())
				collectCtxCancel()
			}

			// skip this tx
			continue
		}

		txSetMtx.Lock()
		txSet.TransactionRefs = append(txSet.TransactionRefs, tx.GetStorageRef())
		txSetMtx.Unlock()

		select {
		case txProcessedNotify <- struct{}{}:
		default:
		}
	}

	// Finally, return new app state ref as in-band.
	ctx = pbobject.WithEncryptionConf(ctx, &p.opts.EncryptionConfig)
	objStore := objstore.GetObjStore(ctx)
	if objStore == nil {
		return nil, errors.New("object store not set, cannot propose blocks")
	}

	if len(txSet.TransactionRefs) < int(p.opts.MinBlockTransactions) {
		for _, tx := range appliedTxs {
			_ = p.mempool.Enqueue(ctx, tx)
		}
		return nil, nil
	}

	stateRef, err := afterState.Serialize()
	if err != nil {
		return nil, err
	}

	encConf := p.opts.EncryptionConfig
	txSetRef, _, err := objStore.StoreObject(ctx, txSet, encConf)
	if err != nil {
		return nil, err
	}

	nextState := &transaction.BlockState{
		ApplicationStateRef: stateRef,
		TransactionSetRef:   txSetRef,
	}

	le := logctx.GetLogEntry(ctx)
	le.
		WithField("tx-count", len(txSet.GetTransactionRefs())).
		Info("proposing block")

	propRef, _, err := objStore.StoreObject(ctx, nextState, encConf)
	if err == nil {
		p.app.Promote(afterState)
	}

	return propRef, err
}
