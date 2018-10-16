package mempool

import (
	"context"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
	"github.com/aperturerobotics/pbobject"
	"github.com/pkg/errors"
)

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

// _ is a type assertion
var _ Orderer = TimestampOrderer
