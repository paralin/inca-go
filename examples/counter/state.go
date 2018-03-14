package main

import (
	"github.com/aperturerobotics/pbobject"
)

// GetObjectTypeID returns the object type string, used to identify types.
func (s *State) GetObjectTypeID() *pbobject.ObjectTypeID {
	return &pbobject.ObjectTypeID{
		TypeUuid: "/inca/example/counter/block_state",
	}
}
