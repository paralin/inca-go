package chain

import (
	"context"
	"encoding/hex"
	"fmt"
	"path"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/inca-go/logctx"
)

// SegmentStore keeps track of Segments in memory.
type SegmentStore struct {
	ctx        context.Context
	le         *logrus.Entry
	ch         *Chain
	segmentMap sync.Map
	dbm        db.Db
	segmentDbm db.Db
}

// NewSegmentStore builds a new SegmentStore.
func NewSegmentStore(ctx context.Context, ch *Chain, dbm db.Db) *SegmentStore {
	le := logctx.GetLogEntry(ctx)
	return &SegmentStore{
		ctx:        ctx,
		ch:         ch,
		dbm:        dbm,
		segmentDbm: db.WithPrefix(dbm, []byte("/segments")),
		le:         le,
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

// GetDigestKey returns the key for the given digest.
func (s *SegmentStore) GetDigestKey(hash []byte) []byte {
	hashHex := hex.EncodeToString(hash)
	return []byte(fmt.Sprintf("/%s", hashHex))
}

// GetBlockSegment returns a segment with the block included.
func (s *SegmentStore) GetBlockSegment(ctx context.Context, blockDigest []byte) (*Segment, error) {
	// TODO
	return nil, nil
}
