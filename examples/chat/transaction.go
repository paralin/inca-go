package main

import (
	"context"

	"github.com/aperturerobotics/inca-go/utils/transaction"
	"github.com/aperturerobotics/pbobject"
)

// GetTransaction follows a transaction refererence.
func GetTransaction(
	ctx context.Context,
	tx *transaction.Transaction,
) (*ChatTransaction, error) {
	var txd ChatTransaction
	if err := tx.GetInnerRef().FollowRef(ctx, nil, &txd, nil); err != nil {
		return nil, err
	}

	return &txd, nil
}

// GetObjectTypeID returns the object type string, used to identify types.
func (c *ChatTransaction) GetObjectTypeID() *pbobject.ObjectTypeID {
	return &pbobject.ObjectTypeID{
		TypeUuid: "/inca/example/chat/transaction",
	}
}
