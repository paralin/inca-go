package segment

import (
	"context"

	"github.com/aperturerobotics/inca"
	"github.com/aperturerobotics/inca-go/block"
	isegment "github.com/aperturerobotics/inca/segment"

	hblock "github.com/aperturerobotics/hydra/block"
	hobject "github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/hydra/object"

	"github.com/golang/protobuf/proto"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Segment is an instance of a connected or disconnected segment of the blockchain.
type Segment struct {
	state      isegment.SegmentState
	ctx        context.Context // Ctx is canceled when the segment is removed from memory
	store      object.ObjectStore
	rootCursor *hobject.Cursor
	le         *logrus.Entry
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
func (s *Segment) GetHeadBlock() *cid.BlockRef {
	return s.state.GetHeadBlock()
}

// GetTailBlock returns the reference to the tail block.
func (s *Segment) GetTailBlock() *cid.BlockRef {
	return s.state.GetTailBlock()
}

// GetTailBlockRound returns the reference to the tail round information.
func (s *Segment) GetTailBlockRound() *inca.BlockRoundInfo {
	return s.state.GetTailBlockRound()
}

// dbkey returns the database key of this segment.
func (s *Segment) dbKey() string {
	return s.state.GetId()
}

// writeState writes the state to the database.
func (s *Segment) writeState(ctx context.Context) error {
	dat, err := proto.Marshal(&s.state)
	if err != nil {
		return err
	}

	return s.store.SetObject(s.dbKey(), dat)
}

// readState reads the state from the database.
// Note: the state object must be allocated, and the ID set.
// If the key does not exist nothing happens.
func (s *Segment) readState(ctx context.Context) error {
	dat, datOk, err := s.store.GetObject(s.dbKey())
	if err != nil || !datOk {
		return err
	}

	return proto.Unmarshal(dat, &s.state)
}

// AppendBlock attempts to append a block to the segment.
func (s *Segment) AppendBlock(
	ctx context.Context,
	blkRef *cid.BlockRef,
	blk *block.Block,
	blkParent *block.Block,
	blkValidator block.Validator,
) error {
	_, bcursor := s.rootCursor.BuildTransaction(nil)
	blkParentNextRef := blkParent.GetNextBlock()
	if blkParentNextRef != nil {
		blkParentNextCursor, err := bcursor.FollowRef(0, blkParentNextRef)
		if err != nil {
			return err
		}
		blkParentNexti, err := blkParentNextCursor.Unmarshal(func() hblock.Block { return &inca.Block{} })
		if err != nil {
			return err
		}
		blkParentNext := blkParentNexti.(*inca.Block)

		if !blkParentNext.GetBlockHeaderRef().EqualsRef(blk.GetInnerBlock().GetBlockHeaderRef()) {
			// TODO: resolve fork choice
			return errors.Errorf("fork: block A: %s and B: %s", blk.GetId(), blkParent.GetId())
		}

		// block is already appended
		return nil
	}

	if s.state.GetSegmentNext() != "" {
		return errors.Errorf("fork: next segment already exists")
	}

	isValid, err := blkParent.ValidateChild(ctx, blk, blkValidator)
	if err != nil {
		return err
	}

	blkParent.NextBlock = blkRef
	if err := blkParent.WriteState(s.store); err != nil {
		return err
	}

	blk.SegmentId = blkParent.SegmentId
	if err := blk.WriteState(s.store); err != nil {
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
	blockValidator block.Validator,
	blockDbm object.ObjectStore,
) (retErr error) {
	if s.state.GetStatus() != isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		return nil
	}

	defer func() {
		s.le.
			WithField("segment", s.GetId()).
			WithField("segment-status", s.state.GetStatus().String()).
			WithField("tail-height", s.state.GetTailBlockRound().String()).
			WithField("error", retErr).
			Debug("rewound once")
	}()

	tailRef := s.state.GetTailBlock()
	_, tailBlkCursor := s.rootCursor.BuildTransactionAtRef(nil, tailRef)
	tailBlk, err := block.GetBlock(ctx, tailBlkCursor, blockDbm)
	if err != nil {
		return err
	}

	tailBlkHeader := tailBlk.GetHeader()
	prevBlockRef := tailBlkHeader.GetPrevBlockRef()
	prevBlkCursor, err := tailBlkCursor.FollowRef(4, prevBlockRef)
	if err != nil {
		return err
	}
	prevBlk, err := block.GetBlock(
		ctx,
		prevBlkCursor,
		blockDbm,
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
			blockValidator,
			blockDbm,
		)
	}

	// Include in this segment
	prevBlk.SegmentId = s.GetId()
	prevBlk.NextBlock = tailBlk.GetBlockRef()
	if err := prevBlk.WriteState(blockDbm); err != nil {
		return err
	}

	s.state.TailBlock = prevBlockRef
	s.state.TailBlockRound = prevBlk.GetHeader().GetRoundInfo()

	isValid, err := prevBlk.ValidateChild(ctx, tailBlk, blockValidator)
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
	blockValidator block.Validator,
	blockDbm object.ObjectStore,
) error {
	if segNext.state.GetStatus() != isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		return errors.Errorf("unexpected status: %v", segNext.state.GetStatus())
	}

	tailRef := segNext.state.GetTailBlock()
	_, tailCursor := s.rootCursor.BuildTransactionAtRef(nil, tailRef)
	tailBlk, err := block.GetBlock(
		ctx,
		tailCursor,
		blockDbm,
	)
	if err != nil {
		return err
	}

	sHeadRef := s.state.GetHeadBlock()
	_, headCursor := s.rootCursor.BuildTransactionAtRef(nil, sHeadRef)
	sHeadBlk, err := block.GetBlock(
		ctx,
		headCursor,
		blockDbm,
	)
	if err != nil {
		return err
	}

	isValid, err := sHeadBlk.ValidateChild(ctx, tailBlk, blockValidator)
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

// MarkValid marks the segment as valid.
func (s *Segment) MarkValid() error {
	if s.state.Status == isegment.SegmentStatus_SegmentStatus_VALID {
		return nil
	}

	s.state.Status = isegment.SegmentStatus_SegmentStatus_VALID
	return s.writeState(s.ctx)
}
