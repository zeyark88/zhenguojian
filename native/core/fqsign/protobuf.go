package fqsign

import "encoding/binary"

// 仅编码的 protobuf 写入器（x-argus 用）。
// writeVarint 先掩到 32 位，与上游字节布局一致。对照 fqapp rust/src/sign/protobuf.rs。

type protoField struct {
	idx    int32
	typ    byte
	vUint  uint64
	vBytes int
}

// ProtoBuf 有序 protobuf Bean。
type ProtoBuf struct {
	fields []protoField
	blobs  [][]byte
}

// NewProtoBuf 创建空 Bean。
func NewProtoBuf() *ProtoBuf { return &ProtoBuf{} }

// PutVarint 写入 varint 字段。
func (p *ProtoBuf) PutVarint(idx int32, v int64) {
	if v < 0 {
		// 负数按 u64 环绕（与 i64 as u64 一致）。
		p.fields = append(p.fields, protoField{idx, 0, uint64(v), 0})
		return
	}
	p.fields = append(p.fields, protoField{idx, 0, uint64(v), 0})
}

func (p *ProtoBuf) putBlob(idx int32, data []byte) {
	p.blobs = append(p.blobs, data)
	p.fields = append(p.fields, protoField{idx, 2, 0, len(p.blobs) - 1})
}

// PutUTF8 写入字符串字段。
func (p *ProtoBuf) PutUTF8(idx int32, s string) { p.putBlob(idx, []byte(s)) }

// PutBytes 写入字节字段。
func (p *ProtoBuf) PutBytes(idx int32, data []byte) { p.putBlob(idx, data) }

// PutProtobuf 写入嵌套消息字段。
func (p *ProtoBuf) PutProtobuf(idx int32, nested *ProtoBuf) { p.putBlob(idx, nested.ToBuf()) }

// ToBuf 序列化。
func (p *ProtoBuf) ToBuf() []byte {
	var out []byte
	for _, f := range p.fields {
		key := uint64(f.idx)<<3 | uint64(f.typ&7)
		out = writeVarint(out, key)
		switch f.typ {
		case 5:
			var b [4]byte
			binary.LittleEndian.PutUint32(b[:], uint32(f.vUint))
			out = append(out, b[:]...)
		case 1:
			var b [8]byte
			binary.LittleEndian.PutUint64(b[:], f.vUint)
			out = append(out, b[:]...)
		case 0:
			out = writeVarint(out, f.vUint)
		case 2:
			blob := p.blobs[f.vBytes]
			out = writeVarint(out, uint64(len(blob)))
			out = append(out, blob...)
		}
	}
	return out
}

func writeVarint(dst []byte, v uint64) []byte {
	v &= 0xFFFFFFFF
	for v >= 128 {
		dst = append(dst, byte(v&127)|128)
		v >>= 7
	}
	return append(dst, byte(v&127))
}
