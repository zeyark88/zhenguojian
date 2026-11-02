package fqsign

import (
	crand "crypto/rand"
	mrand "math/rand"
	"sync"
	"time"
)

// 工具：随机与 query 解析（对照 fqapp sign/mod.rs）。

var seedOnce sync.Once

func ensureSeed() {
	seedOnce.Do(func() {
		mrand.New(mrand.NewSource(time.Now().UnixNano()))
	})
}

func randomInt32() int32 {
	var b [4]byte
	if _, err := crand.Read(b[:]); err == nil {
		v := int32(b[0]) | int32(b[1])<<8 | int32(b[2])<<16 | int32(b[3])<<24
		if v != 0 {
			return v
		}
	}
	ensureSeed()
	v := mrand.Int31() | 1
	if v == 0 {
		return 1
	}
	return v
}

// RandomBytes 生成 n 个随机字节。
func RandomBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := crand.Read(b); err != nil {
		ensureSeed()
		for i := range b {
			b[i] = byte(mrand.Intn(256))
		}
	}
	return b
}

// QueryParam 从原始 query 里取参数（不做百分号解码——签名只对原始串）。
func QueryParam(queryString, name string) (string, bool) {
	if queryString == "" {
		return "", false
	}
	for _, pair := range splitQuery(queryString) {
		k, v := pair, ""
		if i := indexByte(pair, '='); i >= 0 {
			k, v = pair[:i], pair[i+1:]
		}
		if k == name {
			return v, true
		}
	}
	return "", false
}

func queryParamDefault(queryString, name string) string {
	v, ok := QueryParam(queryString, name)
	if !ok {
		return ""
	}
	return v
}

func splitQuery(q string) []string {
	out := make([]string, 0, 16)
	start := 0
	for i := 0; i < len(q); i++ {
		if q[i] == '&' {
			out = append(out, q[start:i])
			start = i + 1
		}
	}
	return append(out, q[start:])
}

func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}
