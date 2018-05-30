package mempool

import (
	"github.com/aperturerobotics/inca"
)

// Application manages state and transaction verification and processing.
type Application interface {
	// PrePoolCheckTx checks a transaction before placing it into the pool.
	// SysErr indicates if the returned error is a system error (as opposed to a rejection of the tx)
	// If Err is nil the value of SysErr is ignored and the TX is assumed to be valid.
	PrePoolCheckTx(txMessage *inca.NodeMessage) (sysErr bool, err error)
}
