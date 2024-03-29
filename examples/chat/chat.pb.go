// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/aperturerobotics/inca-go/examples/chat/chat.proto

package main

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

// ChatState is the application state.
type ChatState struct {
	MessageCount         uint32   `protobuf:"varint,1,opt,name=message_count,json=messageCount" json:"message_count,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ChatState) Reset()         { *m = ChatState{} }
func (m *ChatState) String() string { return proto.CompactTextString(m) }
func (*ChatState) ProtoMessage()    {}
func (*ChatState) Descriptor() ([]byte, []int) {
	return fileDescriptor_chat_956b1e53ec78dab6, []int{0}
}
func (m *ChatState) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ChatState.Unmarshal(m, b)
}
func (m *ChatState) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ChatState.Marshal(b, m, deterministic)
}
func (dst *ChatState) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChatState.Merge(dst, src)
}
func (m *ChatState) XXX_Size() int {
	return xxx_messageInfo_ChatState.Size(m)
}
func (m *ChatState) XXX_DiscardUnknown() {
	xxx_messageInfo_ChatState.DiscardUnknown(m)
}

var xxx_messageInfo_ChatState proto.InternalMessageInfo

func (m *ChatState) GetMessageCount() uint32 {
	if m != nil {
		return m.MessageCount
	}
	return 0
}

// ChatTransaction is a chat transaction.
type ChatTransaction struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ChatTransaction) Reset()         { *m = ChatTransaction{} }
func (m *ChatTransaction) String() string { return proto.CompactTextString(m) }
func (*ChatTransaction) ProtoMessage()    {}
func (*ChatTransaction) Descriptor() ([]byte, []int) {
	return fileDescriptor_chat_956b1e53ec78dab6, []int{1}
}
func (m *ChatTransaction) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ChatTransaction.Unmarshal(m, b)
}
func (m *ChatTransaction) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ChatTransaction.Marshal(b, m, deterministic)
}
func (dst *ChatTransaction) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChatTransaction.Merge(dst, src)
}
func (m *ChatTransaction) XXX_Size() int {
	return xxx_messageInfo_ChatTransaction.Size(m)
}
func (m *ChatTransaction) XXX_DiscardUnknown() {
	xxx_messageInfo_ChatTransaction.DiscardUnknown(m)
}

var xxx_messageInfo_ChatTransaction proto.InternalMessageInfo

func init() {
	proto.RegisterType((*ChatState)(nil), "main.ChatState")
	proto.RegisterType((*ChatTransaction)(nil), "main.ChatTransaction")
}

func init() {
	proto.RegisterFile("github.com/aperturerobotics/inca-go/examples/chat/chat.proto", fileDescriptor_chat_956b1e53ec78dab6)
}

var fileDescriptor_chat_956b1e53ec78dab6 = []byte{
	// 150 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x2c, 0xcc, 0xb1, 0x0a, 0xc2, 0x30,
	0x10, 0xc6, 0x71, 0x0a, 0x22, 0x18, 0x2c, 0x62, 0x27, 0x47, 0xa9, 0x8b, 0x8b, 0x8d, 0xe0, 0xea,
	0xd6, 0x37, 0x50, 0x77, 0xb9, 0x1e, 0x47, 0x12, 0x30, 0xb9, 0x90, 0xbb, 0x80, 0x8f, 0x2f, 0x2d,
	0x2e, 0xdf, 0xf0, 0x83, 0xef, 0x6f, 0xee, 0x2e, 0xa8, 0xaf, 0xd3, 0x80, 0x1c, 0x2d, 0x64, 0x2a,
	0x5a, 0x0b, 0x15, 0x9e, 0x58, 0x03, 0x8a, 0x0d, 0x09, 0xe1, 0xe2, 0xd8, 0xd2, 0x17, 0x62, 0xfe,
	0x90, 0x58, 0xf4, 0xa0, 0xcb, 0x0c, 0xb9, 0xb0, 0x72, 0xb7, 0x8a, 0x10, 0x52, 0x7f, 0x35, 0x9b,
	0xd1, 0x83, 0x3e, 0x15, 0x94, 0xba, 0x93, 0x69, 0x23, 0x89, 0x80, 0xa3, 0x37, 0x72, 0x4d, 0x7a,
	0x68, 0x8e, 0xcd, 0xb9, 0x7d, 0x6c, 0xff, 0x38, 0xce, 0xd6, 0xef, 0xcd, 0x6e, 0x7e, 0xbc, 0x0a,
	0x24, 0x01, 0xd4, 0xc0, 0x69, 0x5a, 0x2f, 0xc5, 0xdb, 0x2f, 0x00, 0x00, 0xff, 0xff, 0xf3, 0x0d,
	0x87, 0xb7, 0x91, 0x00, 0x00, 0x00,
}
