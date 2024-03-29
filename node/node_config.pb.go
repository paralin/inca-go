// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/aperturerobotics/inca-go/node/node_config.proto

package node

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

// Config is the node config.
type Config struct {
	// PrivKey is the private key of the node.
	PrivKey              []byte   `protobuf:"bytes,1,opt,name=priv_key,json=privKey,proto3" json:"priv_key,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Config) Reset()         { *m = Config{} }
func (m *Config) String() string { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()    {}
func (*Config) Descriptor() ([]byte, []int) {
	return fileDescriptor_node_config_9d4b70ef114072fc, []int{0}
}
func (m *Config) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Config.Unmarshal(m, b)
}
func (m *Config) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Config.Marshal(b, m, deterministic)
}
func (dst *Config) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Config.Merge(dst, src)
}
func (m *Config) XXX_Size() int {
	return xxx_messageInfo_Config.Size(m)
}
func (m *Config) XXX_DiscardUnknown() {
	xxx_messageInfo_Config.DiscardUnknown(m)
}

var xxx_messageInfo_Config proto.InternalMessageInfo

func (m *Config) GetPrivKey() []byte {
	if m != nil {
		return m.PrivKey
	}
	return nil
}

func init() {
	proto.RegisterType((*Config)(nil), "node.Config")
}

func init() {
	proto.RegisterFile("github.com/aperturerobotics/inca-go/node/node_config.proto", fileDescriptor_node_config_9d4b70ef114072fc)
}

var fileDescriptor_node_config_9d4b70ef114072fc = []byte{
	// 121 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0xb2, 0x4a, 0xcf, 0x2c, 0xc9,
	0x28, 0x4d, 0xd2, 0x4b, 0xce, 0xcf, 0xd5, 0x4f, 0x2c, 0x48, 0x2d, 0x2a, 0x29, 0x2d, 0x4a, 0x2d,
	0xca, 0x4f, 0xca, 0x2f, 0xc9, 0x4c, 0x2e, 0xd6, 0xcf, 0xcc, 0x4b, 0x4e, 0xd4, 0x4d, 0xcf, 0xd7,
	0xcf, 0xcb, 0x4f, 0x49, 0x05, 0x13, 0xf1, 0xc9, 0xf9, 0x79, 0x69, 0x99, 0xe9, 0x7a, 0x05, 0x45,
	0xf9, 0x25, 0xf9, 0x42, 0x2c, 0x20, 0x21, 0x25, 0x65, 0x2e, 0x36, 0x67, 0xb0, 0xa8, 0x90, 0x24,
	0x17, 0x47, 0x41, 0x51, 0x66, 0x59, 0x7c, 0x76, 0x6a, 0xa5, 0x04, 0xa3, 0x02, 0xa3, 0x06, 0x4f,
	0x10, 0x3b, 0x88, 0xef, 0x9d, 0x5a, 0x99, 0xc4, 0x06, 0xd6, 0x61, 0x0c, 0x08, 0x00, 0x00, 0xff,
	0xff, 0xac, 0x07, 0x56, 0x4e, 0x6f, 0x00, 0x00, 0x00,
}
