package fqsign

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
	"strconv"
)

// x-argus：有序 protobuf Bean -> PKCS7 -> Simon -> 组帧 -> AES-CBC。
// 逐字节对照 fqapp rust/src/sign/argus.rs 移植。

var (
	argusSignKeyHex = "fc78e0a9657a0c748ce51559903ccf03510e51d3cff232d71343e88a321c5304"
	argusAESKeyHex  = "d31e3718288a1027baab59f146a09a9c"
	argusAESIVHex   = "ea180a0336ed352fcd24e4d50018ae54"
)

func mustHex(s string) []byte {
	b, err := hex.DecodeString(s)
	if err != nil {
		panic("fqsign: bad hex: " + err.Error())
	}
	return b
}

func sm3Prefix6(msg []byte) []byte {
	h := Sm3Hash(msg)
	return h[:6]
}

// ArgusBodyHash getBodyHash(stub)。
func ArgusBodyHash(stubHex string) []byte {
	if stubHex == "" {
		return sm3Prefix6(make([]byte, 16))
	}
	raw, err := hex.DecodeString(stubHex)
	if err != nil {
		raw = nil
	}
	return sm3Prefix6(raw)
}

// ArgusQueryHash getQueryHash(query)。
func ArgusQueryHash(query string) []byte {
	if query == "" {
		return sm3Prefix6(make([]byte, 16))
	}
	return sm3Prefix6([]byte(query))
}

// ArgusEncryptEncPb encryptEncPb($data, $length)，length = len(data)+8。
func ArgusEncryptEncPb(data []byte, length int) []byte {
	buf := make([]byte, length)
	n := len(data)
	if n > length {
		n = length
	}
	copy(buf[:n], data[:n])
	xorKey := data[:8]
	for i := 8; i < length; i++ {
		src := byte(0)
		if i < len(data) {
			src = data[i]
		}
		buf[i] = src ^ xorKey[i%8]
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf
}

func simonKeyList() [4]uint64 {
	key := mustHex(argusSignKeyHex)
	var out [4]uint64
	for i := 0; i < 4; i++ {
		b := key[i*8 : i*8+8]
		b8 := [8]byte{}
		copy(b8[:], b)
		out[i] = le64(b8)
	}
	return out
}

func le64(b [8]byte) uint64 {
	var v uint64
	for i := 0; i < 8; i++ {
		v |= uint64(b[i]) << uint(i*8)
	}
	return v
}

func simonEncryptPadded(protobuf []byte) []byte {
	key := simonKeyList()
	encPb := make([]byte, len(protobuf))
	for i := 0; i < len(protobuf)/16; i++ {
		block := protobuf[i*16 : i*16+16]
		var a, b [8]byte
		copy(a[:], block[:8])
		copy(b[:], block[8:])
		ct := SimonEnc([2]uint64{le64(a), le64(b)}, key)
		var ca, cb [8]byte
		for j := 0; j < 8; j++ {
			ca[j] = byte(ct[0] >> uint(j*8) & 255)
			cb[j] = byte(ct[1] >> uint(j*8) & 255)
		}
		copy(encPb[i*16:i*16+8], ca[:])
		copy(encPb[i*16+8:i*16+16], cb[:])
	}
	return encPb
}

func pkcs7Align(data *[]byte) {
	padLen := 16 - len(*data)%16
	for i := 0; i < padLen; i++ {
		*data = append(*data, byte(padLen))
	}
}

func aesCbcEncrypt(data, key, iv []byte) []byte {
	if len(data)%16 != 0 {
		panic("fqsign: aes-cbc input not block aligned")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		panic("fqsign: aes: " + err.Error())
	}
	out := make([]byte, len(data))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(out, data)
	return out
}

// ArgusEncrypt 旧式（PHP55 / SignatureManager）加密路径。
func ArgusEncrypt(bean *ProtoBuf) string {
	protobuf := bean.ToBuf()
	pkcs7Align(&protobuf)
	newLen := len(protobuf)

	encPb := simonEncryptPadded(protobuf)
	bBuffer := ArgusEncryptEncPb(encPb, newLen+8)
	bBuffer = append(bBuffer, 'a', 'o')

	pkcs7Align(&bBuffer)
	out := aesCbcEncrypt(bBuffer, mustHex(argusAESKeyHex), mustHex(argusAESIVHex))
	return base64.StdEncoding.EncodeToString(out)
}

var (
	argus551Prefix = [2]byte{0xf2, 0x81}
	argus551Magic8 = [8]byte{0xf2, 0xf7, 0xfc, 0xff, 0xf2, 0xf7, 0xfc, 0xff}
	argus551Tail9  = [9]byte{0xea, 0xfb, 0x2c, 0xfe, 0x85, 0x68, 0x51, 0x91, 0x54}
)

// ArgusEncrypt551 番茄上游真正接受的 x-argus 包装。
func ArgusEncrypt551(bean *ProtoBuf) string {
	protobuf := bean.ToBuf()
	pkcs7Align(&protobuf)

	encPb := simonEncryptPadded(protobuf)

	un := make([]byte, 0, 8+len(encPb)+9)
	un = append(un, argus551Magic8[:]...)
	un = append(un, encPb...)
	un = append(un, argus551Tail9[:]...)

	buf := make([]byte, len(un))
	copy(buf[:8], argus551Magic8[:])
	for i := 8; i < len(un); i++ {
		buf[i] = un[i] ^ argus551Magic8[i%8]
	}
	// reverse
	for i, j := 0, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	buf = append(buf, 'a', 'o')

	pkcs7Align(&buf)
	out := aesCbcEncrypt(buf, mustHex(argusAESKeyHex), mustHex(argusAESIVHex))

	raw := append([]byte{}, argus551Prefix[:]...)
	raw = append(raw, out...)
	return base64.StdEncoding.EncodeToString(raw)
}

// ArgusStyle Bean 布局选择。
type ArgusStyle int

const (
	// ArgusStylePhp55 旧布局。
	ArgusStylePhp55 ArgusStyle = iota
	// ArgusStyleSigMgr SignatureManager 布局。
	ArgusStyleSigMgr
	// ArgusStyle551 当前布局：字段 8-11 加 551 组帧。
	ArgusStyle551
)

// ArgusParams 构建 x-argus Bean 的运行时输入。
type ArgusParams struct {
	QueryString string
	StubHex     string
	Timestamp   int64
	Aid         int64
	LicenseID   int64
	DeviceID    string
	// InstallID 非空时写入 Bean 字段 16。
	InstallID string
	Style     ArgusStyle
	RandField int32
}

// ArgusGetSign 生成 x-argus 值。
func ArgusGetSign(p ArgusParams) string {
	aid := p.Aid
	if aid == 0 {
		// aid 必须与请求 query 的 aid 一致（红果 8662 / 番茄 1967）。
		// 漏传会导致签名静默不匹配，这里要求显式传入。
		panic("fqsign: ArgusGetSign requires non-zero aid matching the request query")
	}
	licenseID := p.LicenseID
	if licenseID == 0 {
		licenseID = 1611921764
	}
	randField := p.RandField
	if randField == 0 {
		randField = randomInt32()
	}

	bean := NewProtoBuf()
	bean.PutVarint(1, 1077940818)
	bean.PutVarint(2, 2)
	bean.PutVarint(3, int64(randField))
	bean.PutUTF8(4, itoa64(aid))
	bean.PutUTF8(5, p.DeviceID)
	bean.PutUTF8(6, itoa64(licenseID))
	bean.PutUTF8(7, queryParamDefault(p.QueryString, "version_name"))
	switch p.Style {
	case ArgusStyle551:
		bean.PutUTF8(8, "v04.04.05-ov-android")
		bean.PutVarint(9, 134744640)
		bean.PutBytes(10, make([]byte, 8))
		bean.PutVarint(11, 0)
	case ArgusStyleSigMgr:
		bean.PutUTF8(8, "v04.04.05-ov-android")
		bean.PutVarint(9, 134744640)
		bean.PutUTF8(10, "")
		bean.PutUTF8(11, "0")
	default:
		bean.PutUTF8(8, "")
		bean.PutVarint(9, 0)
		bean.PutUTF8(10, "")
		bean.PutUTF8(11, "")
	}
	bean.PutVarint(12, p.Timestamp<<1)
	bean.PutBytes(13, ArgusBodyHash(p.StubHex))
	bean.PutBytes(14, ArgusQueryHash(p.QueryString))

	sub15 := NewProtoBuf()
	sub15.PutVarint(1, 1)
	sub15.PutVarint(2, 1)
	sub15.PutVarint(3, 1)
	sub15.PutVarint(7, 0xC792ECCC)
	bean.PutProtobuf(15, sub15)

	bean.PutUTF8(16, p.InstallID)
	bean.PutUTF8(20, "none")
	bean.PutVarint(21, 738)

	sub23 := NewProtoBuf()
	sub23.PutUTF8(1, "NX551J")
	sub23.PutVarint(2, 8196)
	sub23.PutVarint(4, 0x80E0D800)
	bean.PutProtobuf(23, sub23)

	bean.PutVarint(25, 2)

	if p.Style == ArgusStyle551 {
		return ArgusEncrypt551(bean)
	}
	return ArgusEncrypt(bean)
}

func itoa64(v int64) string { return strconv.FormatInt(v, 10) }
