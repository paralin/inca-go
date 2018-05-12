package state

import (
	"context"
	"time"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/storageref"
)

// ChainStateSnapshot is an instance of the chain state.
type ChainStateSnapshot struct {
	// Context will be canceled when the state is invalidated.
	Context context.Context `json:"-"`
	// RoundStarted indicates if the round is in progress.
	RoundStarted bool
	// BlockRoundInfo is the current block height and round information.
	BlockRoundInfo *inca.BlockRoundInfo
	// LastBlockHeader is the last block header.
	LastBlockHeader *inca.BlockHeader
	// LastBlockRef is the reference to the last block.
	LastBlockRef *storageref.StorageRef
	// CurrentProposer contains a reference to the computed current proposer if known.
	CurrentProposer *inca.Validator
	// RoundEndTime is the time when the round will end.
	RoundEndTime time.Time
	// RoundStartTime is the time when the round will/did start.
	RoundStartTime time.Time
	// NextRoundStartTime is the time when the next round will start.
	NextRoundStartTime time.Time
	// TotalVotingPower is the current total amount of voting power.
	TotalVotingPower int32
	// ChainConfig is the current chain config.
	ChainConfig *inca.ChainConfig
	// ValidatorSet is the current validator set.
	ValidatorSet *inca.ValidatorSet
}
