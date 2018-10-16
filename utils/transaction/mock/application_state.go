package mock

import (
	"context"
	"errors"

	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/inca-go/utils/transaction/mempool"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/aperturerobotics/storageref"
)

// VirtualState is an application state snapshot.
type VirtualState struct {
	state AppState
}

// PrePoolCheckTx checks a transaction before placing it into the pool.
// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
func (v *VirtualState) PrePoolCheckTx(
	ctx context.Context,
	tx *transaction.Transaction,
) (sysErr bool, err error) {
	var txInner Transaction
	if err := tx.GetNodeMessage().GetInnerRef().FollowRef(ctx, nil, &txInner, nil); err != nil {
		return true, err
	}

	if len(txInner.GetInnerData()) == 0 {
		return false, errors.New("transaction empty")
	}

	return false, nil
}

// Apply applies a transaction to the state.
// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
func (v *VirtualState) Apply(
	ctx context.Context,
	tx *transaction.Transaction,
) (sysErr bool, txErr error) {
	var txInner Transaction
	if err := tx.GetNodeMessage().GetInnerRef().FollowRef(ctx, nil, &txInner, nil); err != nil {
		return true, err
	}

	v.state.AppliedTxCount++
	v.state.AppliedTxSize += uint32(len(txInner.GetInnerData()))
	v.state.LastTxData = make([]byte, len(txInner.GetInnerData()))
	copy(v.state.LastTxData, txInner.GetInnerData())
	return false, nil
}

// Serialize returns a state reference that can be used in a block.
// If the state is already serialized, should return the same ref.
func (v *VirtualState) Serialize(ctx context.Context) (*storageref.StorageRef, error) {
	objStore := objstore.GetObjStore(ctx)
	if objStore == nil {
		return nil, errors.New("no objstore attached to context")
	}

	encConf := pbobject.GetEncryptionConf(ctx)
	if encConf == nil {
		encConf = &pbobject.EncryptionConfig{}
	}

	sref, _, err := objStore.StoreObject(ctx, &v.state, *encConf)
	return sref, err
}

// _ is a type assertion.
var _ mempool.ApplicationState = ((*VirtualState)(nil))
