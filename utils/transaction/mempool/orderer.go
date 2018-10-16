package mempool

import (
	"context"

	"github.com/aperturerobotics/inca-go/utils/transaction/txdb"
)

// Orderer determines the priority value for a transaction.
type Orderer func(
	ctx context.Context,
	txDb *txdb.TxDatabase,
	txID string,
) (float64, error)
