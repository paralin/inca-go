package mock

import (
	"github.com/aperturerobotics/pbobject"
)

// GetObjectTypeID returns the object type string, used to identify types.
func (t *Transaction) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/util/transaction/mock/tx")
}

// GetObjectTypeID returns the object type string, used to identify types.
func (s *AppState) GetObjectTypeID() *pbobject.ObjectTypeID {
	return pbobject.NewObjectTypeID("/inca/util/transaction/mock/app-state")
}
