package chain

import (
	"context"
	"path"
	"sync"

	"github.com/aperturerobotics/inca-go/db"
)

// SegmentStore keeps track of Segments in memory.
type SegmentStore struct {
	ctx        context.Context
	ch         *Chain
	segmentMap sync.Map
	dbm        db.Db
}

// NewSegmentStore builds a new SegmentStore.
func NewSegmentStore(ctx context.Context, ch *Chain, dbm db.Db) *SegmentStore {
	return &SegmentStore{
		ctx: ctx,
		ch:  ch,
		dbm: dbm,
	}
}

// dbListSegments lists segment IDs in the database.
func (s *SegmentStore) dbListSegments(ctx context.Context) ([]string, error) {
	keys, err := s.dbm.List(ctx, []byte("/"))
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

	seg := &Segment{}
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
