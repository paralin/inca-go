package chain

import (
	"context"
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/aperturerobotics/inca-go/db"
	"github.com/aperturerobotics/objstore"
	"github.com/aperturerobotics/pbobject"
	"github.com/golang/protobuf/proto"
)

// Segment is an instance of a connected segment of the blockchain.
type Segment struct {
	state SegmentState
	ctx   context.Context       // Ctx is canceled when the segment is removed from memory
	db    *objstore.ObjectStore // Db is the object store
	dbm   db.Db                 // Dbm is the local key/value store
	chain *Chain                // Chain is a reference to the parent blockchain
	le    *logrus.Entry         // le is the logger
}

// GetId returns the identifier of this segment.
func (s *Segment) GetId() string {
	return s.state.GetId()
}

// GetStatus returns the status of the segment.
func (s *Segment) GetStatus() SegmentStatus {
	return s.state.GetStatus()
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

// GetObjectTypeID returns the object type string, used to identify types.
func (s *SegmentState) GetObjectTypeID() *pbobject.ObjectTypeID {
	return &pbobject.ObjectTypeID{TypeUuid: "/inca/segment"}
}
