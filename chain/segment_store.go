package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"sync"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/logctx"
	ichain "github.com/aperturerobotics/inca/chain"

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
	ch              *Chain
	segmentMap      sync.Map // map[string]*Segment (by id)
	dbm             db.Db
	segmentDbm      db.Db
	objStore        *objstore.ObjectStore
	segmentQueue    *SegmentQueue
	segmentNotifyCh <-chan *Segment
}

// NewSegmentStore builds a new SegmentStore.
func NewSegmentStore(
	ctx context.Context,
	ch *Chain,
	dbm db.Db,
	objStore *objstore.ObjectStore,
) *SegmentStore {
	le := logctx.GetLogEntry(ctx)
	segmentNotifyCh := make(chan *Segment, 5)
	return &SegmentStore{
		ctx:             ctx,
		ch:              ch,
		dbm:             dbm,
		segmentDbm:      db.WithPrefix(dbm, []byte("/segments")),
		objStore:        objStore,
		le:              le,
		segmentNotifyCh: segmentNotifyCh,
		segmentQueue:    NewSegmentQueue(segmentNotifyCh),
	}
}

// manageSegmentStore manages the segment store.
func (s *SegmentStore) manageSegmentStore(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.segmentNotifyCh:
		}

		s.rewindOnce(ctx)
	}
}

// rewindOnce rewinds the highest priority segment by one.
func (s *SegmentStore) rewindOnce(ctx context.Context) {
	nextSeg := s.segmentQueue.Pop()
	if nextSeg == nil {
		return
	}

	if err := nextSeg.RewindOnce(ctx, s); err != nil {
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

	if nextSeg.state.GetStatus() == ichain.SegmentStatus_SegmentStatus_DISJOINTED {
		s.segmentQueue.Push(nextSeg)
	}
}

// dbListSegments lists segment IDs in the database.
func (s *SegmentStore) dbListSegments(ctx context.Context) ([]string, error) {
	keys, err := s.segmentDbm.List(ctx, []byte("/"))
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
		dbm: s.segmentDbm,
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
		dbm: db.WithPrefix(s.dbm, []byte("/segments")),
		le:  le,
	}

	seg.state.Id = segmentID
	seg.state.HeadBlock = blkRef
	seg.state.Status = ichain.SegmentStatus_SegmentStatus_DISJOINTED
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
