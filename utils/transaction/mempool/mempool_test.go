package mempool

import (
	"context"
	"testing"

	"github.com/aperturerobotics/objstore/db/inmem"
	// "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMempool tests the mempool.
func TestMempool(t *testing.T) {
	ctx := context.Background()
	dbm := inmem.NewInmemDb()

	m, err := NewMempool(ctx, dbm, Opts{})
	require.NoError(t, err)
	_ = m
}
