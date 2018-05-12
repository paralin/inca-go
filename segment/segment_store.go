package segment

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"sync"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/encryption"
	"github.com/aperturerobotics/inca-go/logctx"
	isegment "github.com/aperturerobotics/inca/segment"

	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/objstore/db"
	"github.com/aperturerobotics/storageref"

	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// SegmentStore keeps track of Segments in memory.
type SegmentStore struct {
	ctx             context.Context
	le              *logrus.Entry
	segmentMap      sync.Map // map[string]*Segment (by id)
	dbm             db.Db
	objStore        *objstore.ObjectStore
	segmentQueue    *SegmentQueue
	segmentNotifyCh <-chan *Segment
}

// NewSegmentStore builds a new SegmentStore, with a management goroutine.
func NewSegmentStore(
	ctx context.Context,
	dbm db.Db,
	objStore *objstore.ObjectStore,
	encStrat encryption.Strategy,
	blockValidator block.Validator,
	blockDbm db.Db,
) *SegmentStore {
	le := logctx.GetLogEntry(ctx)
	segmentNotifyCh := make(chan *Segment, 5)
	ss := &SegmentStore{
		ctx:             ctx,
		dbm:             dbm,
		objStore:        objStore,
		le:              le,
		segmentNotifyCh: segmentNotifyCh,
		segmentQueue:    NewSegmentQueue(segmentNotifyCh),
	}
	go ss.manageSegmentStore(ctx, encStrat, blockValidator, blockDbm)
	return ss
}

// manageSegmentStore manages the segment store.
func (s *SegmentStore) manageSegmentStore(
	ctx context.Context,
	encStrat encryption.Strategy,
	blockValidator block.Validator,
	blockDbm db.Db,
) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.segmentNotifyCh:
		}

		s.rewindOnce(ctx, encStrat, blockValidator, blockDbm)
	}
}

// rewindOnce rewinds the highest priority segment by one.
func (s *SegmentStore) rewindOnce(
	ctx context.Context,
	encStrat encryption.Strategy,
	blockValidator block.Validator,
	blockDbm db.Db,
) {
	nextSeg := s.segmentQueue.Pop()
	if nextSeg == nil {
		return
	}

	if err := nextSeg.RewindOnce(ctx, s, encStrat, blockValidator, blockDbm); err != nil {
		s.le.WithError(err).Warn("issue rewinding segment")
		return
	}

	if prevSegID := nextSeg.state.GetSegmentPrev(); prevSegID != "" {
		prevSeg, err := s.GetSegmentById(ctx, prevSegID)
		if err != nil {
			s.le.WithError(err).Warn("issue rewinding parent of segment")
			return
		}

		nextSeg = prevSeg
	}

	if nextSeg.state.GetStatus() == isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		s.segmentQueue.Push(nextSeg)
	}
}

// dbListSegments lists segment IDs in the database.
func (s *SegmentStore) dbListSegments(ctx context.Context) ([]string, error) {
	keys, err := s.dbm.List(ctx, nil)
	if err != nil {
		return nil, err
	}

	res := make([]string, len(keys))
	for i, key := range keys {
		keyPath := string(key)
		res[i] = path.Base(keyPath)
	}

	return res, nil
}

// GetSegmentById returns the segment by Id if it exists.
func (s *SegmentStore) GetSegmentById(ctx context.Context, id string) (*Segment, error) {
	segInter, ok := s.segmentMap.Load(id)
	if ok {
		return segInter.(*Segment), nil
	}

	le := logctx.GetLogEntry(ctx)
	seg := &Segment{
		ctx: ctx,
		db:  s.objStore,
		dbm: s.dbm,
		le:  le,
	}

	seg.state.Id = id
	if err := seg.readState(ctx); err != nil {
		return nil, err
	}

	segAct, loaded := s.segmentMap.LoadOrStore(id, seg)
	/*
		if !loaded {
			// manage state
		}
	*/
	if loaded {
		seg = segAct.(*Segment)
	}
	return seg, nil
}

// GetDigestKey returns the key for the given digest.
func (s *SegmentStore) GetDigestKey(hash []byte) []byte {
	hashHex := hex.EncodeToString(hash)
	return []byte(fmt.Sprintf("/%s", hashHex))
}

// NewSegment builds a new segment for a block.
func (s *SegmentStore) NewSegment(ctx context.Context, blk *block.Block, blkRef *storageref.StorageRef) (*Segment, error) {
	uid, _ := uuid.NewV4()
	segmentID := uid.String()
	le := logctx.GetLogEntry(ctx)
	seg := &Segment{
		ctx: ctx,
		db:  s.objStore,
		dbm: s.dbm,
		le:  le,
	}

	seg.state.Id = segmentID
	seg.state.HeadBlock = blkRef
	seg.state.Status = isegment.SegmentStatus_SegmentStatus_DISJOINTED
	seg.state.TailBlock = blkRef
	seg.state.TailBlockRound = blk.GetHeader().GetRoundInfo()
	if err := seg.writeState(ctx); err != nil {
		return nil, err
	}

	blk.State.SegmentId = segmentID
	if err := blk.WriteState(ctx); err != nil {
		return nil, err
	}

	// return the singleton
	return s.GetSegmentById(ctx, segmentID)
}
