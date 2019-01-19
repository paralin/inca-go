// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/aperturerobotics/inca-go/utils/transaction/transaction.proto

package transaction

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import cid "github.com/aperturerobotics/hydra/cid"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// BlockState is the in-band data stored in the BlockHeader app state pointer.
type BlockState struct {
	// ApplicationStateRef is the reference to the application state.
	ApplicationStateRef *cid.BlockRef `protobuf:"bytes,1,opt,name=application_state_ref,json=applicationStateRef" json:"application_state_ref,omitempty"`
	// TransactionSetRef is the reference to the transaction set.
	TransactionSetRef    *cid.BlockRef `protobuf:"bytes,2,opt,name=transaction_set_ref,json=transactionSetRef" json:"transaction_set_ref,omitempty"`
	XXX_NoUnkeyedLiteral struct{}      `json:"-"`
	XXX_unrecognized     []byte        `json:"-"`
	XXX_sizecache        int32         `json:"-"`
}

func (m *BlockState) Reset()         { *m = BlockState{} }
func (m *BlockState) String() string { return proto.CompactTextString(m) }
func (*BlockState) ProtoMessage()    {}
func (*BlockState) Descriptor() ([]byte, []int) {
	return fileDescriptor_transaction_f67ce095c1514782, []int{0}
}
func (m *BlockState) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BlockState.Unmarshal(m, b)
}
func (m *BlockState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BlockState.Marshal(b, m, deterministic)
}
func (dst *BlockState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BlockState.Merge(dst, src)
}
func (m *BlockState) XXX_Size() int {
	return xxx_messageInfo_BlockState.Size(m)
}
func (m *BlockState) XXX_DiscardUnknown() {
	xxx_messageInfo_BlockState.DiscardUnknown(m)
}

var xxx_messageInfo_BlockState proto.InternalMessageInfo

func (m *BlockState) GetApplicationStateRef() *cid.BlockRef {
	if m != nil {
		return m.ApplicationStateRef
	}
	return nil
}

func (m *BlockState) GetTransactionSetRef() *cid.BlockRef {
	if m != nil {
		return m.TransactionSetRef
	}
	return nil
}

// TransactionSet contains an ordered set of transactions applied to the state.
type TransactionSet struct {
	// TransactionRefs contains NodeMessage references in the order they were applied.
	TransactionRefs      []*cid.BlockRef `protobuf:"bytes,1,rep,name=transaction_refs,json=transactionRefs" json:"transaction_refs,omitempty"`
	XXX_NoUnkeyedLiteral struct{}        `json:"-"`
	XXX_unrecognized     []byte          `json:"-"`
	XXX_sizecache        int32           `json:"-"`
}

func (m *TransactionSet) Reset()         { *m = TransactionSet{} }
func (m *TransactionSet) String() string { return proto.CompactTextString(m) }
func (*TransactionSet) ProtoMessage()    {}
func (*TransactionSet) Descriptor() ([]byte, []int) {
	return fileDescriptor_transaction_f67ce095c1514782, []int{1}
}
func (m *TransactionSet) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_TransactionSet.Unmarshal(m, b)
}
func (m *TransactionSet) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_TransactionSet.Marshal(b, m, deterministic)
}
func (dst *TransactionSet) XXX_Merge(src proto.Message) {
	xxx_messageInfo_TransactionSet.Merge(dst, src)
}
func (m *TransactionSet) XXX_Size() int {
	return xxx_messageInfo_TransactionSet.Size(m)
}
func (m *TransactionSet) XXX_DiscardUnknown() {
	xxx_messageInfo_TransactionSet.DiscardUnknown(m)
}

var xxx_messageInfo_TransactionSet proto.InternalMessageInfo

func (m *TransactionSet) GetTransactionRefs() []*cid.BlockRef {
	if m != nil {
		return m.TransactionRefs
	}
	return nil
}

func init() {
	proto.RegisterType((*BlockState)(nil), "transaction.BlockState")
	proto.RegisterType((*TransactionSet)(nil), "transaction.TransactionSet")
}

func init() {
	proto.RegisterFile("github.com/aperturerobotics/inca-go/utils/transaction/transaction.proto", fileDescriptor_transaction_f67ce095c1514782)
}

var fileDescriptor_transaction_f67ce095c1514782 = []byte{
	// 225 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x7c, 0x8f, 0x3d, 0x4b, 0x04, 0x31,
	0x10, 0x86, 0x59, 0x05, 0x8b, 0x1c, 0x7e, 0xe5, 0x10, 0x0e, 0xab, 0xe3, 0x2a, 0x1b, 0x37, 0xa0,
	0x8d, 0x8d, 0x85, 0x36, 0x82, 0x65, 0xce, 0x7e, 0x99, 0xcd, 0x4d, 0x6e, 0x83, 0xeb, 0x4e, 0x98,
	0xcc, 0x16, 0xfe, 0x09, 0x7f, 0xb3, 0x24, 0x36, 0x11, 0xe5, 0x8a, 0x81, 0xe1, 0xe5, 0x79, 0x1f,
	0x66, 0xd4, 0xcb, 0x3e, 0xc8, 0x30, 0xf7, 0xad, 0xa3, 0x0f, 0x03, 0x11, 0x59, 0x66, 0x46, 0xa6,
	0x9e, 0x24, 0xb8, 0x64, 0xc2, 0xe4, 0xe0, 0x76, 0x4f, 0x66, 0x96, 0x30, 0x26, 0x23, 0x0c, 0x53,
	0x02, 0x27, 0x81, 0xa6, 0x7a, 0x6f, 0x23, 0x93, 0x90, 0x5e, 0x54, 0xd1, 0xb5, 0x39, 0x64, 0x1d,
	0x3e, 0x77, 0x0c, 0xc6, 0x85, 0x5d, 0x9e, 0x9f, 0xf6, 0xe6, 0xab, 0x51, 0xea, 0x79, 0x24, 0xf7,
	0xbe, 0x15, 0x10, 0xd4, 0x4f, 0xea, 0x0a, 0x62, 0x1c, 0x83, 0x83, 0xac, 0xeb, 0x52, 0x0e, 0x3b,
	0x46, 0xbf, 0x6a, 0xd6, 0xcd, 0xcd, 0xe2, 0xee, 0xb4, 0xcd, 0xcd, 0xc2, 0x5b, 0xf4, 0x76, 0x59,
	0xb1, 0xa5, 0x6f, 0xd1, 0xeb, 0x47, 0xb5, 0xac, 0x2e, 0xea, 0x12, 0x4a, 0x11, 0x1c, 0xfd, 0x27,
	0xb8, 0xac, 0xc8, 0x2d, 0x8a, 0x45, 0xbf, 0x79, 0x55, 0x67, 0x6f, 0xbf, 0x42, 0xfd, 0xa0, 0x2e,
	0x6a, 0x21, 0xa3, 0x4f, 0xab, 0x66, 0x7d, 0xfc, 0xd7, 0x76, 0x5e, 0x61, 0x16, 0x7d, 0xea, 0x4f,
	0xca, 0x8f, 0xf7, 0xdf, 0x01, 0x00, 0x00, 0xff, 0xff, 0xed, 0x65, 0x7c, 0x10, 0x6c, 0x01, 0x00,
	0x00,
}
