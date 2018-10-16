package mempool

import (
	"context"
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
	ctx     context.Context
	opts    ProposerOpts
	mempool *Mempool
	app     Application
}

// NewProposer builds a new proposer.
// Context controls cancellation globally.
func NewProposer(
	ctx context.Context,
	memPool *Mempool,
	opts ProposerOpts,
	app Application,
) (*Proposer, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	return &Proposer{
		ctx:     ctx,
		opts:    opts,
		mempool: memPool,
		app:     app,
	}, nil
}

// ProposerOpts are options passed to the mempool proposer.
type ProposerOpts struct {
	// ProposeDeadlineRatio is the max percentage of round duration to use for gathering transactions.
	// After this deadline, if at least one transaction was found, it will be processed in the block.
	// If this is set to 1, the proposal will be fired after the wait ratio.
	// If this is set to 0, a default of 0.5 is used.
	// TODO: If EmitEmptyBlocks is set, this timing will also be used to emit the empty block.
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

	le := logctx.GetLogEntry(ctx)
	ctx = pbobject.WithEncryptionConf(ctx, &p.opts.EncryptionConfig)
	objStore := objstore.GetObjStore(ctx)
	if objStore == nil {
		return nil, errors.New("object store not set, cannot propose blocks")
	}

	// Pull the previous block state.
	var blkState transaction.BlockState
	err := parentBlk.
		GetStateRef().
		FollowRef(ctx, nil, &blkState, nil)
	if err != nil {
		return nil, errors.Wrap(err, "parent block state lookup")
	}

	roundDur := chainState.RoundEndTime.Sub(chainState.RoundStartTime)
	waitUntil := chainState.RoundStartTime.Add(time.Duration(p.opts.ProposeWaitRatio) * roundDur)
	if deadlineRatio := p.opts.ProposeDeadlineRatio; deadlineRatio != 0 {
		deadlineTime := chainState.RoundStartTime.Add(roundDur * time.Duration(deadlineRatio))
		collectCtx, collectCtxCancel = context.WithDeadline(ctx, deadlineTime)
	} else {
		collectCtx, collectCtxCancel = context.WithCancel(ctx)
	}
	defer collectCtxCancel()

	// Stage: collect transaction IDs
	collectedTxIds := make(chan string)
	errCh := make(chan error, 3)
	go func() {
		// Kick this off early to start the fibheap computations.
		errCh <- p.mempool.CollectTransactions(
			collectCtx,
			int(p.opts.MaxBlockTranasctions),
			collectedTxIds,
		)
		close(collectedTxIds)
	}()

	// Stage: resolve transactions (dequeue from pool)
	resolvedTxs := make(chan *transaction.Transaction)
	pendingState, err := p.app.GetStateAtBlock(ctx, parentBlk, &blkState)
	if err != nil {
		return nil, errors.Wrap(err, "parent app state lookup")
	}

	go func() {
		defer close(resolvedTxs)

		txDb := p.mempool.GetTransactionDb()
		for {
			var txID string
			var ok bool
			select {
			case txID, ok = <-collectedTxIds:
				if !ok {
					return
				}
			case <-collectCtx.Done():
				return
			}

			tx, err := txDb.GetTx(collectCtx, txID)
			if err != nil {
				le.WithError(err).Warn("unable to lookup tx")
				if err == context.Canceled || err == context.DeadlineExceeded {
					_ = p.mempool.Enqueue(p.ctx, txID)
				}
				continue
			}

			sysErr, txErr := pendingState.PrePoolCheckTx(ctx, tx)
			if txErr != nil {
				if sysErr || txErr == context.Canceled || txErr == context.DeadlineExceeded {
					errCh <- txErr
					_ = p.mempool.Enqueue(p.ctx, txID)
					collectCtxCancel()
				}
				return
			}

			select {
			case <-collectCtx.Done():
				_ = p.mempool.Enqueue(p.ctx, txID)
				return
			case resolvedTxs <- tx:
			}
		}
	}()

	// Stage: apply transactions.
	var appliedTxs []string
	requeueTxs := func() {
		for _, txID := range appliedTxs {
			_ = p.mempool.Enqueue(p.ctx, txID)
		}
	}

	txSet := &transaction.TransactionSet{}
TxLoop:
	for {
		var tx *transaction.Transaction
		var ok bool
		select {
		case err := <-errCh:
			if err != nil {
				if len(appliedTxs) == 0 && err == context.Canceled {
					return nil, nil
				}

				requeueTxs()
				return nil, err
			}
			continue
		case <-ctx.Done():
			requeueTxs()
			return nil, ctx.Err()
		case tx, ok = <-resolvedTxs:
			if !ok {
				break TxLoop
			}
		}

		appliedTxs = append(appliedTxs, tx.GetID())
		sysErr, txErr := pendingState.Apply(ctx, tx)
		if txErr != nil {
			if sysErr {
				requeueTxs()
				return nil, txErr
			}

			appliedTxs = appliedTxs[:len(appliedTxs)-1]
			continue
		}

		txSet.TransactionRefs = append(txSet.TransactionRefs, tx.GetNodeMessageRef())

		txCount := int32(len(appliedTxs))
		hasMinTxs := txCount > p.opts.MinBlockTransactions
		if p.opts.MinBlockTransactions == 0 {
			hasMinTxs = txCount != 0
		}

		hasMaxTxs := txCount >= p.opts.MaxBlockTranasctions
		waitDurElapsed := time.Now().After(waitUntil)
		if hasMinTxs && (hasMaxTxs || waitDurElapsed) {
			break
		}
	}

	select {
	case err := <-errCh:
		if err != nil {
			return nil, err
		}
	default:
	}

	le = le.
		WithField("tx-count", len(txSet.GetTransactionRefs()))

	// Finally, return new app state ref as in-band.
	if len(txSet.TransactionRefs) < int(p.opts.MinBlockTransactions) {
		// Abstain, not enough transactions.
		requeueTxs()

		le = le.WithField("min-tx-count", p.opts.MinBlockTransactions)
		le.Debug("abstaining, not enough transactions")
		return nil, nil
	}

	stateRef, err := pendingState.Serialize(ctx)
	if err != nil {
		requeueTxs()
		return nil, err
	}

	encConf := p.opts.EncryptionConfig
	txSetRef, _, err := objStore.StoreObject(ctx, txSet, encConf)
	if err != nil {
		requeueTxs()
		return nil, err
	}

	nextState := &transaction.BlockState{
		ApplicationStateRef: stateRef,
		TransactionSetRef:   txSetRef,
	}

	propRef, _, err := objStore.StoreObject(ctx, nextState, encConf)
	if err != nil {
		requeueTxs()
		return nil, err
	}

	return propRef, nil
}
