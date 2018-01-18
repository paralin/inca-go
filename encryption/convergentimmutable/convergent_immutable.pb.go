// Code generated by protoc-gen-go. DO NOT EDIT.
// source: github.com/aperturerobotics/inca-go/encryption/convergentimmutable/convergent_immutable.proto

/*
Package convergentimmutable is a generated protocol buffer package.

It is generated from these files:
	github.com/aperturerobotics/inca-go/encryption/convergentimmutable/convergent_immutable.proto

It has these top-level messages:
	Config
*/
package convergentimmutable

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

// Config is the configuration for the convergent immutable strategy.
type Config struct {
	// PreSharedKey is the 32 byte pre-shared key.
	PreSharedKey []byte `protobuf:"bytes,1,opt,name=pre_shared_key,json=preSharedKey,proto3" json:"pre_shared_key,omitempty"`
}

func (m *Config) Reset()                    { *m = Config{} }
func (m *Config) String() string            { return proto.CompactTextString(m) }
func (*Config) ProtoMessage()               {}
func (*Config) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Config) GetPreSharedKey() []byte {
	if m != nil {
		return m.PreSharedKey
	}
	return nil
}

func init() {
	proto.RegisterType((*Config)(nil), "convergentimmutable.Config")
}

func init() {
	proto.RegisterFile("github.com/aperturerobotics/inca-go/encryption/convergentimmutable/convergent_immutable.proto", fileDescriptor0)
}

var fileDescriptor0 = []byte{
	// 155 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x6c, 0xcc, 0x31, 0xcb, 0xc2, 0x30,
	0x10, 0x80, 0x61, 0xba, 0x74, 0x28, 0xe5, 0x1b, 0xfa, 0x2d, 0x8e, 0x22, 0x0e, 0x2e, 0x36, 0x83,
	0x3f, 0xc1, 0xd1, 0x4d, 0x67, 0x09, 0x49, 0x3c, 0xd3, 0x43, 0x73, 0x17, 0xae, 0x17, 0x21, 0xff,
	0x5e, 0xe8, 0x20, 0x0e, 0xae, 0x0f, 0x2f, 0x6f, 0x77, 0x8d, 0xa8, 0x53, 0xf1, 0x63, 0xe0, 0x64,
	0x5c, 0x06, 0xd1, 0x22, 0x20, 0xec, 0x59, 0x31, 0xcc, 0x06, 0x29, 0xb8, 0x7d, 0x64, 0x03, 0x14,
	0xa4, 0x66, 0x45, 0x26, 0x13, 0x98, 0x5e, 0x20, 0x11, 0x48, 0x31, 0xa5, 0xa2, 0xce, 0x3f, 0xe1,
	0xcb, 0xec, 0x07, 0xc7, 0x2c, 0xac, 0x3c, 0xfc, 0xff, 0xe8, 0x37, 0x63, 0xd7, 0x1e, 0x99, 0xee,
	0x18, 0x87, 0x6d, 0xf7, 0x97, 0x05, 0xec, 0x3c, 0x39, 0x81, 0x9b, 0x7d, 0x40, 0x5d, 0x35, 0xeb,
	0x66, 0xd7, 0x9f, 0xfb, 0x2c, 0x70, 0x59, 0xf0, 0x04, 0xd5, 0xb7, 0xcb, 0xeb, 0xf0, 0x0e, 0x00,
	0x00, 0xff, 0xff, 0xcf, 0xc3, 0x47, 0x78, 0xac, 0x00, 0x00, 0x00,
}
