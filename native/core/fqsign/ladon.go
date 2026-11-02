package fqsign

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// x-ladon：34 轮 ARX 分组密码（u64）。
// 逐字节对照 fqapp rust/src/sign/ladon.rs 移植。

// LadonExpandHashTable 由 32 字符小写 md5 hex 构建 288 字节哈希表。
func LadonExpandHashTable(md5Hex string) [288]byte {
	var ht [288]byte
	bytes := []byte(md5Hex)
	prefixLen := 32
	if len(bytes) < prefixLen {
		prefixLen = len(bytes)
	}
	copy(ht[:prefixLen], bytes[:prefixLen])

	load := func(off int) uint64 {
		var v uint64
		for j := 0; j < 8; j++ {
			v |= uint64(ht[off+j]) << uint(j*8)
		}
		return v
	}

	temp := make([]uint64, 0, 4)
	for i := 0; i < 4; i++ {
		temp = append(temp, load(i*8))
	}
	bufB0 := temp[0]
	bufB8 := temp[1]
	temp = temp[2:]

	for i := 0; i < 34; i++ {
		x9 := bufB0
		x8 := bufB8

		x8 = rotr64(x8, 8)
		x8 += x9
		x8 ^= uint64(i)
		temp = append(temp, x8)

		rorX9 := rotl64(x9, 3)
		x8 ^= rorX9

		for j := 0; j < 8; j++ {
			ht[(i+1)*8+j] = byte(x8 >> uint(j*8) & 255)
		}

		bufB0 = x8
		bufB8 = temp[0]
		temp = temp[1:]
	}
	return ht
}

// LadonEncryptBlock 加密一个 16 字节块。
func LadonEncryptBlock(ht *[288]byte, block []byte) [16]byte {
	var data0, data1 uint64
	for i := 0; i < 8; i++ {
		data0 |= uint64(block[i]) << uint(i*8)
		data1 |= uint64(block[8+i]) << uint(i*8)
	}

	for i := 0; i < 34; i++ {
		var hash uint64
		for j := 0; j < 8; j++ {
			hash |= uint64(ht[i*8+j]) << uint(j*8)
		}
		rotated := rotr64(data1, 8)
		sum := data0 + rotated
		data1 = hash ^ sum

		rotated = rotl64(data0, 3)
		data0 = data1 ^ rotated
	}

	var out [16]byte
	for i := 0; i < 8; i++ {
		out[i] = byte(data0 >> uint(i*8) & 255)
		out[8+i] = byte(data1 >> uint(i*8) & 255)
	}
	return out
}

// LadonEncryptRaw 用 md5 hex 做密钥加密任意数据（PKCS 式填充）。
func LadonEncryptRaw(md5Hex string, data []byte) []byte {
	ht := LadonExpandHashTable(md5Hex)

	size := len(data)
	newSize := size + (16 - size%16)
	padVal := byte(newSize - size)
	input := make([]byte, newSize)
	for i := range input {
		input[i] = padVal
	}
	copy(input, data)

	out := make([]byte, 0, newSize)
	for i := 0; i < newSize/16; i++ {
		block := LadonEncryptBlock(&ht, input[i*16:i*16+16])
		out = append(out, block[:]...)
	}
	return out
}

// LadonEncrypt 由 khronos、lcId、aid 和 4 个随机字节生成 x-ladon 值。
func LadonEncrypt(khronos, lcID, aid int64, randomBytes []byte) string {
	data := fmt.Sprintf("%d-%d-%d", khronos, lcID, aid)

	keygen := append(append([]byte{}, randomBytes...), []byte(fmt.Sprintf("%d", aid))...)
	sum := md5.Sum(keygen)
	md5Hex := hex.EncodeToString(sum[:])

	encrypted := LadonEncryptRaw(md5Hex, []byte(data))
	out := append(append([]byte{}, randomBytes...), encrypted...)
	return base64.StdEncoding.EncodeToString(out)
}
