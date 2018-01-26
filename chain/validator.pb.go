// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/aperturerobotics/inca-go/chain/validator.proto

package chain

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import inca "github.com/aperturerobotics/inca"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// ValidatorState is state for a validator.
type ValidatorState struct {
	LastVote *inca.BlockRoundInfo `protobuf:"bytes,1,opt,name=last_vote,json=lastVote" json:"last_vote,omitempty"`
}

func (m *ValidatorState) Reset()                    { *m = ValidatorState{} }
func (m *ValidatorState) String() string            { return proto.CompactTextString(m) }
func (*ValidatorState) ProtoMessage()               {}
func (*ValidatorState) Descriptor() ([]byte, []int) { return fileDescriptor4, []int{0} }

func (m *ValidatorState) GetLastVote() *inca.BlockRoundInfo {
	if m != nil {
		return m.LastVote
	}
	return nil
}

func init() {
	proto.RegisterType((*ValidatorState)(nil), "chain.ValidatorState")
}

func init() {
	proto.RegisterFile("github.com/aperturerobotics/inca-go/chain/validator.proto", fileDescriptor4)
}

var fileDescriptor4 = []byte{
	// 158 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xb2, 0x4c, 0xcf, 0x2c, 0xc9,
	0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x4f, 0x2c, 0x48, 0x2d, 0x2a, 0x29, 0x2d, 0x4a, 0x2d,
	0xca, 0x4f, 0xca, 0x2f, 0xc9, 0x4c, 0x2e, 0xd6, 0xcf, 0xcc, 0x4b, 0x4e, 0xd4, 0x4d, 0xcf, 0xd7,
	0x4f, 0xce, 0x48, 0xcc, 0xcc, 0xd3, 0x2f, 0x4b, 0xcc, 0xc9, 0x4c, 0x49, 0x2c, 0xc9, 0x2f, 0xd2,
	0x2b, 0x28, 0xca, 0x2f, 0xc9, 0x17, 0x62, 0x05, 0x0b, 0x4b, 0x69, 0x13, 0x32, 0x01, 0x4c, 0x40,
	0xf4, 0x28, 0x39, 0x73, 0xf1, 0x85, 0xc1, 0x8c, 0x09, 0x2e, 0x49, 0x2c, 0x49, 0x15, 0x32, 0xe4,
	0xe2, 0xcc, 0x49, 0x2c, 0x2e, 0x89, 0x2f, 0xcb, 0x2f, 0x49, 0x95, 0x60, 0x54, 0x60, 0xd4, 0xe0,
	0x36, 0x12, 0xd1, 0x03, 0xeb, 0x70, 0xca, 0xc9, 0x4f, 0xce, 0x0e, 0xca, 0x2f, 0xcd, 0x4b, 0xf1,
	0xcc, 0x4b, 0xcb, 0x0f, 0xe2, 0x00, 0x29, 0x0b, 0xcb, 0x2f, 0x49, 0x4d, 0x62, 0x03, 0x9b, 0x65,
	0x0c, 0x08, 0x00, 0x00, 0xff, 0xff, 0x24, 0x56, 0x3f, 0x23, 0xbc, 0x00, 0x00, 0x00,
}