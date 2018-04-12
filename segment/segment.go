package segment

import (
	"context"
	"fmt"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/encryption"
	isegment "github.com/aperturerobotics/inca/segment"

	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/storageref"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Segment is an instance of a connected or disconnected segment of the blockchain.
type Segment struct {
	state isegment.SegmentState
	ctx   context.Context       // Ctx is canceled when the segment is removed from memory
	db    *objstore.ObjectStore // Db is the object store
	dbm   db.Db                 // Dbm is the local key/value store
	le    *logrus.Entry         // le is the logger
}

// GetId returns the identifier of this segment.
func (s *Segment) GetId() string {
	return s.state.GetId()
}

// GetStatus returns the status of the segment.
func (s *Segment) GetStatus() isegment.SegmentStatus {
	return s.state.GetStatus()
}

// GetSegmentNext returns the ID of the next segment.
func (s *Segment) GetSegmentNext() string {
	return s.state.GetSegmentNext()
}

// GetHeadBlock returns the reference to the head block.
func (s *Segment) GetHeadBlock() *storageref.StorageRef {
	return s.state.GetHeadBlock()
}

// dbkey returns the database key of this segment.
func (s *Segment) dbKey() []byte {
	return []byte(fmt.Sprintf("/%s", s.state.GetId()))
}

// writeState writes the state to the database.
func (s *Segment) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&s.state)
	if err != nil {
		return err
	}

	return s.dbm.Set(ctx, s.dbKey(), dat)
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (s *Segment) readState(ctx context.Context) error {
	dat, err := s.dbm.Get(ctx, s.dbKey())
	if err != nil {
		return err
	}

	if len(dat) == 0 {
		return nil
	}

	return proto.Unmarshal(dat, &s.state)
}

// AppendBlock attempts to append a block to the segment.
func (s *Segment) AppendBlock(
	ctx context.Context,
	blkRef *storageref.StorageRef,
	blk *block.Block,
	blkParent *block.Block,
	encStrat encryption.Strategy,
) error {
	blkParentNextRef := blkParent.GetNextBlock()
	if blkParentNextRef != nil {
		blkParentNext, err := block.FollowBlockRef(ctx, blkParentNextRef, encStrat)
		if err != nil {
			return err
		}

		if !blkParentNext.BlockHeaderRef.Equals(blk.GetInnerBlock().GetBlockHeaderRef()) {
			// TODO: resolve fork choice
			return errors.Errorf("fork: block A: %s and B: %s", blk.GetId(), blkParent.GetId())
		}

		// block is already appended
		return nil
	}

	if s.state.GetSegmentNext() != "" {
		return errors.Errorf("fork: next segment already exists")
	}

	isValid, err := blkParent.ValidateChild(ctx, blk)
	if err != nil {
		return err
	}

	blkParent.NextBlock = blkRef
	if err := blkParent.WriteState(ctx); err != nil {
		return err
	}

	blk.SegmentId = blkParent.SegmentId
	if err := blk.WriteState(ctx); err != nil {
		return err
	}

	s.state.HeadBlock = blkRef
	if isValid && s.state.Status != isegment.SegmentStatus_SegmentStatus_INVALID {
		s.state.Status = isegment.SegmentStatus_SegmentStatus_VALID
	}

	if err := s.writeState(ctx); err != nil {
		return err
	}

	return nil
}

// RewindOnce rewinds the segment once.
func (s *Segment) RewindOnce(
	ctx context.Context,
	segStore *SegmentStore,
	encStrat encryption.Strategy,
	blockValidator block.Validator,
	blockDbm db.Db,
) (retErr error) {
	if s.state.GetStatus() != isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		return nil
	}

	defer func() {
		s.le.
			WithField("segment", s.GetId()).
			WithField("tail-height", s.state.GetTailBlockRound().String()).
			WithField("error", retErr).
			Debug("rewound once")
	}()

	tailRef := s.state.GetTailBlock()
	tailBlk, err := block.FollowBlockRef(ctx, tailRef, encStrat)
	if err != nil {
		return err
	}

	tailBlkObj, err := block.GetBlock(
		ctx,
		encStrat,
		blockValidator,
		blockDbm,
		tailRef,
	)
	if err != nil {
		return err
	}

	tailBlkHeader, err := block.FollowBlockHeaderRef(ctx, tailBlk.GetBlockHeaderRef(), encStrat)
	if err != nil {
		return err
	}

	prevBlockRef := tailBlkHeader.GetLastBlockRef()
	prevBlk, err := block.GetBlock(
		ctx,
		encStrat,
		blockValidator,
		blockDbm,
		prevBlockRef,
	)
	if err != nil {
		return err
	}

	prevSegmentID := prevBlk.GetSegmentId()
	if prevSegmentID != "" {
		prevSegment, err := segStore.GetSegmentById(ctx, prevSegmentID)
		if err != nil {
			return err
		}

		if prevSegment == s {
			return errors.New("previous segment resolved to same segment")
		}

		return prevSegment.AppendSegment(
			ctx,
			s,
			encStrat,
			blockValidator,
			blockDbm,
		)
	}

	// Include in this segment
	prevBlk.SegmentId = s.GetId()
	prevBlk.NextBlock = tailBlkObj.GetBlockRef()
	if err := prevBlk.WriteState(ctx); err != nil {
		return err
	}

	s.state.TailBlock = prevBlockRef
	s.state.TailBlockRound = prevBlk.GetHeader().GetRoundInfo()

	isValid, err := prevBlk.ValidateChild(ctx, tailBlkObj)
	if err != nil {
		s.le.WithError(err).Warn("segment rewound to invalid block, marking as invalid")
		s.state.Status = isegment.SegmentStatus_SegmentStatus_INVALID
	}

	if isValid && s.state.Status != isegment.SegmentStatus_SegmentStatus_INVALID {
		s.le.Info("one or more validators verified this block, marking segment as valid")
		s.state.Status = isegment.SegmentStatus_SegmentStatus_VALID
	}

	/* Re-implement this check in the event handler in chain.
	if s.state.Status == isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		prevBlkRound := prevBlk.GetHeader().GetRoundInfo()
		if prevBlkRound.GetHeight() == 0 {
			if prevBlk.GetHeader().GetChainConfigRef().Equals(chain.GetGenesis().GetInitChainConfigRef()) {
				s.le.Info("traversed to the genesis block, marking segment as valid")
				s.state.Status = isegment.SegmentStatus_SegmentStatus_VALID
			} else {
				s.le.Warn("segment terminates at invalid genesis")
				s.state.Status = isegment.SegmentStatus_SegmentStatus_INVALID
			}
		}
	}
	*/

	if err := s.writeState(ctx); err != nil {
		return err
	}

	/*
		if s.state.Status == isegment.SegmentStatus_SegmentStatus_VALID {
			if chain.state.GetStateSegment() == "" ||
				chain.state.GetLastHeight() < s.state.TailBlockRound.GetHeight() {
				chain.state.StateSegment = s.state.Id
				chain.triggerStateRecheck()
			}
		}
	*/

	return nil
}

// AppendSegment attempts to append a segment to the Segment.
func (s *Segment) AppendSegment(
	ctx context.Context,
	segNext *Segment,
	encStrat encryption.Strategy,
	blockValidator block.Validator,
	blockDbm db.Db,
) error {
	if segNext.state.GetStatus() != isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		return errors.Errorf("unexpected status: %v", segNext.state.GetStatus())
	}

	tailRef := segNext.state.GetTailBlock()
	tailBlk, err := block.GetBlock(
		ctx,
		encStrat,
		blockValidator,
		blockDbm,
		tailRef,
	)
	if err != nil {
		return err
	}

	sHeadRef := s.state.GetHeadBlock()
	sHeadBlk, err := block.GetBlock(
		ctx,
		encStrat,
		blockValidator,
		blockDbm,
		sHeadRef,
	)
	if err != nil {
		return err
	}

	isValid, err := sHeadBlk.ValidateChild(ctx, tailBlk)
	if err != nil {
		segNext.state.Status = isegment.SegmentStatus_SegmentStatus_INVALID
		s.le.
			WithError(err).
			WithField("segment", segNext.GetId()).
			Warn("marking would-be child segment as invalid")

		return segNext.writeState(ctx)
	}

	if isValid && segNext.state.Status == isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		segNext.state.Status = isegment.SegmentStatus_SegmentStatus_VALID
	}

	s.le.WithField("next-seg", segNext.GetId()).Debug("appended segment")
	segNext.state.Status = s.state.Status
	segNext.state.SegmentPrev = s.GetId()
	if err := segNext.writeState(ctx); err != nil {
		return err
	}

	s.state.SegmentNext = segNext.GetId()
	if err := s.writeState(ctx); err != nil {
		return err
	}

	return nil
}
