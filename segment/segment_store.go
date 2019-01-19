package segment

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"sync"

	"github.com/aperturerobotics/inca-go/block"
	"github.com/aperturerobotics/inca-go/logctx"
	isegment "github.com/aperturerobotics/inca/segment"

	hobject "github.com/aperturerobotics/hydra/block/object"
	"github.com/aperturerobotics/hydra/cid"
	"github.com/aperturerobotics/hydra/object"

	"github.com/satori/go.uuid"
	"github.com/sirupsen/logrus"
)

// SegmentStore keeps track of Segments in memory.
type SegmentStore struct {
	ctx             context.Context
	le              *logrus.Entry
	segmentMap      sync.Map // map[string]*Segment (by id)
	store           object.ObjectStore
	rootCursor      *hobject.Cursor
	segmentQueue    *SegmentQueue
	segmentNotifyCh <-chan *Segment
	handler         SegmentStoreHandler
}

// SegmentStoreHandler handles segment store events.
type SegmentStoreHandler interface {
	// HandleSegmentUpdated handles a segment update.
	HandleSegmentUpdated(seg *Segment)
}

// NewSegmentStore builds a new SegmentStore, with a management goroutine.
func NewSegmentStore(
	ctx context.Context,
	store object.ObjectStore,
	rootCursor *hobject.Cursor,
	blockValidator block.Validator,
	handler SegmentStoreHandler,
) *SegmentStore {
	le := logctx.GetLogEntry(ctx)
	segmentNotifyCh := make(chan *Segment, 5)
	return &SegmentStore{
		ctx:             ctx,
		le:              le,
		store:           store,
		rootCursor:      rootCursor,
		handler:         handler,
		segmentNotifyCh: segmentNotifyCh,
		segmentQueue:    NewSegmentQueue(segmentNotifyCh),
	}
}

// RewindOnce rewinds the highest priority segment by one.
func (s *SegmentStore) RewindOnce(
	ctx context.Context,
	blockValidator block.Validator,
	blockDbm object.ObjectStore,
) bool {
	nextSeg := s.segmentQueue.Pop()
	if nextSeg == nil {
		return false
	}

	if err := nextSeg.RewindOnce(ctx, s, blockValidator, blockDbm); err != nil {
		s.le.WithError(err).Warn("issue rewinding segment")
		return false
	}

	if prevSegID := nextSeg.state.GetSegmentPrev(); prevSegID != "" {
		prevSeg, err := s.GetSegmentById(ctx, prevSegID)
		if err != nil {
			s.le.WithError(err).Warn("issue rewinding parent of segment")
			return false
		}

		nextSeg = prevSeg
	}

	s.callChangedHandler(nextSeg)

	currStatus := nextSeg.state.GetStatus()
	if currStatus == isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		s.segmentQueue.Push(nextSeg)
	}
	return true
}

// dbListSegments lists segment IDs in the database.
func (s *SegmentStore) dbListSegments(ctx context.Context) ([]string, error) {
	keys, err := s.store.ListKeys("")
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
		ctx:        ctx,
		rootCursor: s.rootCursor,
		store:      s.store,
		le:         le,
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
func (s *SegmentStore) NewSegment(
	ctx context.Context,
	blk *block.Block,
	blkRef *cid.BlockRef,
) (*Segment, error) {
	uid, _ := uuid.NewV4()
	segmentID := uid.String()
	le := logctx.GetLogEntry(ctx)
	seg := &Segment{
		ctx:        ctx,
		store:      s.store,
		rootCursor: s.rootCursor,
		le:         le,
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
	if err := blk.WriteState(s.store); err != nil {
		return nil, err
	}

	// return the singleton
	aseg, err := s.GetSegmentById(ctx, segmentID)
	if err != nil {
		return nil, err
	}

	if aseg.GetStatus() == isegment.SegmentStatus_SegmentStatus_DISJOINTED {
		s.segmentQueue.Push(aseg)
	}

	return aseg, nil
}

// callChangedHandler calls the segment changed handler.
func (s *SegmentStore) callChangedHandler(seg *Segment) {
	if s.handler != nil {
		s.handler.HandleSegmentUpdated(seg)
	}
}
