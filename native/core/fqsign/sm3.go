package fqsign

import "encoding/binary"

// SM3 哈希（国密），逐字节对照 fqapp rust/src/sign/sm3.rs 移植。

var sm3IV = [8]uint32{
	0x7380166f, 0x4914b2b9, 0x172442d7, 0xda8a0600,
	0xa96f30bc, 0x163138aa, 0xe38dee4d, 0xb0fb0e4e,
}

func sm3Tj(j int) uint32 {
	if j < 16 {
		return 0x79cc4519
	}
	return 0x7a879d8a
}

func sm3Rotl(a uint32, k int) uint32 {
	k %= 32
	if k == 0 {
		return a
	}
	return a<<k | a>>(32-k)
}

func sm3FF(x, y, z uint32, j int) uint32 {
	if j < 16 {
		return x ^ y ^ z
	}
	return (x & y) | (x & z) | (y & z)
}

func sm3GG(x, y, z uint32, j int) uint32 {
	if j < 16 {
		return x ^ y ^ z
	}
	return (x & y) | (^x & z)
}

func sm3P0(x uint32) uint32 { return x ^ sm3Rotl(x, 9) ^ sm3Rotl(x, 17) }

func sm3P1(x uint32) uint32 { return x ^ sm3Rotl(x, 15) ^ sm3Rotl(x, 23) }

func sm3CF(v *[8]uint32, block []byte) [8]uint32 {
	var w [68]uint32
	for i := 0; i < 16; i++ {
		w[i] = binary.BigEndian.Uint32(block[i*4:])
	}
	for j := 16; j < 68; j++ {
		w[j] = sm3P1(w[j-16]^w[j-9]^sm3Rotl(w[j-3], 15)) ^ sm3Rotl(w[j-13], 7) ^ w[j-6]
	}
	var w1 [64]uint32
	for j := 0; j < 64; j++ {
		w1[j] = w[j] ^ w[j+4]
	}

	a, b, c, d := v[0], v[1], v[2], v[3]
	e, f, g, h := v[4], v[5], v[6], v[7]

	for j := 0; j < 64; j++ {
		ss1 := sm3Rotl(sm3Rotl(a, 12)+e+sm3Rotl(sm3Tj(j), j), 7)
		ss2 := ss1 ^ sm3Rotl(a, 12)
		tt1 := sm3FF(a, b, c, j) + d + ss2 + w1[j]
		tt2 := sm3GG(e, f, g, j) + h + ss1 + w[j]
		d = c
		c = sm3Rotl(b, 9)
		b = a
		a = tt1
		h = g
		g = sm3Rotl(f, 19)
		f = e
		e = sm3P0(tt2)
	}

	return [8]uint32{
		a ^ v[0], b ^ v[1], c ^ v[2], d ^ v[3],
		e ^ v[4], f ^ v[5], g ^ v[6], h ^ v[7],
	}
}

// Sm3Hash 返回 32 字节 SM3 摘要。
func Sm3Hash(msg []byte) [32]byte {
	origLen := len(msg)
	padded := make([]byte, 0, origLen+72)
	padded = append(padded, msg...)
	padded = append(padded, 0x80)

	reserve := origLen%64 + 1
	rangeEnd := 56
	if rangeEnd < reserve {
		rangeEnd += 64
	}
	for i := reserve; i < rangeEnd; i++ {
		padded = append(padded, 0)
	}
	var lenBytes [8]byte
	binary.BigEndian.PutUint64(lenBytes[:], uint64(origLen)*8)
	padded = append(padded, lenBytes[:]...)

	v := sm3IV
	for i := 0; i < len(padded)/64; i++ {
		v = sm3CF(&v, padded[i*64:i*64+64])
	}

	var out [32]byte
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint32(out[i*4:], v[i])
	}
	return out
}
