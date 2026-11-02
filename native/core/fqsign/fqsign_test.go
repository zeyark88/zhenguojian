package fqsign

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"
	"strings"
	"testing"
)

// 对照 fqapp rust/testdata/vectors.json 的黄金向量（由独立参考实现生成）。
// 移植正确性以这些向量为唯一标准。

type vectors map[string]map[string]any

func mustParse(t *testing.T, s string) uint64 {
	v, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSm3MatchesVectors(t *testing.T) {
	cases := []struct{ name string; input []byte }{
		{"empty", nil},
		{"abc", []byte("abc")},
		{"zero16", make([]byte, 16)},
		{"long", bytesOf('a', 100)},
		{"query", []byte("aid=1967&device_id=123&iid=456")},
	}
	want := map[string]string{
		"empty":  "1ab21d8355cfa17f8e61194831e81a8f22bec8c728fefb747ed035eb5082aa2b",
		"abc":    "66c7f0f462eeedd9d1f2d46bdc10e4e24167c4875cf2f7a2297da02b8f4ba8e0",
		"zero16": "106e34a2b8c7bb13156cfdd0d91379dcc47543dcf9787c68ae5eb582620ae6e8",
		"long":   "0c105d5a46a65fdf0a0938283db2517ea87f176de84786f443cb78802aaa03de",
		"query":  "81c65adb29f5b7912512e81103813432471e6f72311425a111741459d03eee5f",
	}
	for _, c := range cases {
		got := hex.EncodeToString(toByte20(Sm3Hash(c.input)))
		if got != want[c.name] {
			t.Errorf("sm3[%s] = %s, want %s", c.name, got, want[c.name])
		}
	}
}

func TestArgusQueryHashVector(t *testing.T) {
	// vectors.json: argus_queryhash.null = 106e34a2b8c7
	if got := hex.EncodeToString(ArgusQueryHash("")); got != "106e34a2b8c7" {
		t.Errorf("queryhash null = %s", got)
	}
	// argus_queryhash.query("aid=1967&device_id=123") = 925b8d86b20f
	if got := hex.EncodeToString(ArgusQueryHash("aid=1967&device_id=123")); got != "925b8d86b20f" {
		t.Errorf("queryhash = %s", got)
	}
}

func TestArgusBodyHashVector(t *testing.T) {
	if got := hex.EncodeToString(ArgusBodyHash("")); got != "106e34a2b8c7" {
		t.Errorf("bodyhash null = %s", got)
	}
}

func TestSimonEncVector(t *testing.T) {
	pt := [2]uint64{mustParseT(t, "1234567890123456789"), mustParseT(t, "9876543210987654321")}
	k := [4]uint64{
		mustParseT(t, "1111111111111111111"),
		mustParseT(t, "2222222222222222222"),
		mustParseT(t, "3333333333333333333"),
		mustParseT(t, "4444444444444444444"),
	}
	want := [2]uint64{mustParseT(t, "6071150433487837324"), mustParseT(t, "2027321285990159935")}
	got := SimonEnc(pt, k)
	if got != want {
		t.Errorf("simon = %v, want %v", got, want)
	}
}

func TestSimonKeyExpansionVector(t *testing.T) {
	k := [4]uint64{1, 2, 3, 4}
	var key [72]uint64
	copy(key[:4], k[:])
	SimonKeyExpansion(&key)
	wantLast := [4]uint64{mustParseT(t, "13679004903447286117"), mustParseT(t, "6988159389011166349"), mustParseT(t, "12845848347567849394"), mustParseT(t, "14469362449063813004")}
	// 注意：向量文件里 keyexp_last 用的是 1111..., 2222... 那组密钥
	k2 := [4]uint64{
		mustParseT(t, "1111111111111111111"),
		mustParseT(t, "2222222222222222222"),
		mustParseT(t, "3333333333333333333"),
		mustParseT(t, "4444444444444444444"),
	}
	var key2 [72]uint64
	copy(key2[:4], k2[:])
	SimonKeyExpansion(&key2)
	for i := 0; i < 4; i++ {
		if key2[68+i] != wantLast[i] {
			t.Errorf("keyexp[%d] = %d, want %d", 68+i, key2[68+i], wantLast[i])
		}
	}
}

func TestLadonEncryptVector(t *testing.T) {
	rb, _ := hex.DecodeString("01020304")
	got := LadonEncrypt(1700000000, 1611921764, 1967, rb)
	want := "AQIDBKM7Y7bWnOIUXmTybD6/MpGbuge4sBwUJsyHVivhtMsH"
	if got != want {
		t.Errorf("ladon = %s, want %s", got, want)
	}
}

func TestLadonRawVector(t *testing.T) {
	raw := LadonEncryptRaw("53136271c432a1af377c3806c3112ddf", []byte("hello-world-data"))
	want := "995103f804b4fa834266bf7ff4e382563c5745c587456a7ca3cb861c00f487e3"
	if got := hex.EncodeToString(raw); got != want {
		t.Errorf("ladon_raw = %s, want %s", got, want)
	}
}

func TestArgusEncPbVector(t *testing.T) {
	in, _ := hex.DecodeString("1122334411223344112233441122334411223344112233441122334411223344")
	out := ArgusEncryptEncPb(in, len(in)+8)
	want := "4433221144332211000000000000000000000000000000000000000000000000" +
		"4433221144332211"
	if got := hex.EncodeToString(out); got != want {
		t.Errorf("encpb = %s, want %s", got, want)
	}
}

// buildTestBean 与 Rust golden vectors 的确定性 Bean 一致。
func buildTestBean() *ProtoBuf {
	bean := NewProtoBuf()
	bean.PutVarint(1, 1077940818)
	bean.PutVarint(2, 2)
	bean.PutVarint(3, 305419896)
	bean.PutUTF8(4, "1967")
	bean.PutUTF8(5, "test_device_id")
	bean.PutUTF8(6, "1611921764")
	bean.PutUTF8(7, "")
	bean.PutUTF8(8, "")
	bean.PutVarint(9, 0)
	bean.PutUTF8(10, "")
	bean.PutUTF8(11, "")
	bean.PutVarint(12, 1700000000<<1)
	bean.PutBytes(13, ArgusBodyHash(""))
	bean.PutBytes(14, ArgusQueryHash("aid=1967&device_id=test_device_id&iid=456"))

	sub15 := NewProtoBuf()
	sub15.PutVarint(1, 1)
	sub15.PutVarint(2, 1)
	sub15.PutVarint(3, 1)
	sub15.PutVarint(7, 0xC792ECCC)
	bean.PutProtobuf(15, sub15)

	bean.PutUTF8(16, "")
	bean.PutUTF8(20, "none")
	bean.PutVarint(21, 738)

	sub23 := NewProtoBuf()
	sub23.PutUTF8(1, "NX551J")
	sub23.PutVarint(2, 8196)
	sub23.PutVarint(4, 0x80E0D800)
	bean.PutProtobuf(23, sub23)
	bean.PutVarint(25, 2)
	return bean
}

func TestProtobufBeanVector(t *testing.T) {
	// vectors.json: protobuf_bean（注意：该向量 Bean 无 field 25，我们主路径有 25，
	// 因此这里构造一模一样的 Bean 验证）
	bean := NewProtoBuf()
	bean.PutVarint(1, 1077940818)
	bean.PutVarint(2, 2)
	bean.PutVarint(3, 305419896)
	bean.PutUTF8(4, "1967")
	bean.PutUTF8(5, "test_device_id")
	bean.PutUTF8(6, "1611921764")
	bean.PutUTF8(7, "")
	bean.PutUTF8(8, "")
	bean.PutVarint(9, 0)
	bean.PutUTF8(10, "")
	bean.PutUTF8(11, "")
	bean.PutVarint(12, 1700000000<<1)
	bean.PutBytes(13, ArgusBodyHash(""))
	bean.PutBytes(14, ArgusQueryHash("aid=1967&device_id=test_device_id&iid=456"))

	sub15 := NewProtoBuf()
	sub15.PutVarint(1, 1)
	sub15.PutVarint(2, 1)
	sub15.PutVarint(3, 1)
	sub15.PutVarint(7, 0xC792ECCC)
	bean.PutProtobuf(15, sub15)

	bean.PutUTF8(16, "")
	bean.PutUTF8(20, "none")
	bean.PutVarint(21, 738)

	sub23 := NewProtoBuf()
	sub23.PutUTF8(1, "NX551J")
	sub23.PutVarint(2, 8196)
	sub23.PutVarint(4, 0x80E0D800)
	bean.PutProtobuf(23, sub23)
	bean.PutVarint(25, 2)

	want := "08d2a4808204100218f8acd191012204313936372a0e746573745f6465766963655f6964" +
		"320a313631313932313736343a004200480052005a006080c49fd50c6a06106e34a2b8c7" +
		"7206ac5095fc674e7a0c08011001180138ccd9cbbc0c820100a201046e6f6e65a801e205" +
		"ba01110a064e583535314a1084402080b0838708c80102"
	if got := hex.EncodeToString(bean.ToBuf()); got != want {
		t.Errorf("protobuf bean =\n%s\nwant\n%s", got, want)
	}
}

func TestArgusEncryptVector(t *testing.T) {
	// vectors.json: argus_encrypt（field3=305419896 固定，ts=1700000000）
	got := ArgusEncrypt(buildTestBean())
	want := "CenCagJvoxTUWE5EKHSREum5XvTccRUiQ6zMVQ+ukI60LEAcPfL4G+G7JQ+PQUf1N/GhJZfgD" +
		"brDQz6bB5hu2WHD6Jhk+ZJa5OVOITddXAmdw03KsxjyGgg59eQAw/3C77PsT/y36u1YWD9aqegf30VVgrG4doDhNoPZoaTbx4qAzXrwY/gAsrW3zXEgEP2RgepRfPugDCNZPoqFu76/8g=="
	if got != want {
		t.Errorf("argus_encrypt =\n%s\nwant\n%s", got, want)
	}
}

// --- helpers ---

func bytesOf(b byte, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = b
	}
	return out
}

func toByte20(h [32]byte) []byte { return h[:] }

func mustParseT(t *testing.T, s string) uint64 {
	v, err := strconv.ParseUint(strings.TrimSpace(s), 10, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}

var _ = base64.StdEncoding
