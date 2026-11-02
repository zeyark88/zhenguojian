package core

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

// 局域网实测：SSDP 发现（无设备时不失败，只验证不崩、可搜索）。
func TestCastDiscoverLocal(t *testing.T) {
	if testing.Short() {
		t.Skip("short 模式跳过局域网探测")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	devices, err := discoverCastDevices(ctx)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	for _, d := range devices {
		t.Logf("发现: %s | %s | ctrl=%s", d.Name, d.Server, d.ControlURL)
	}
	if len(devices) == 0 {
		t.Log("本网络未发现 DLNA 设备（正常，仅功能验证）")
	}
}

func TestBuildCastSOAPBody(t *testing.T) {
	body := buildCastSOAPBody("http://a/b.m3u8", "A&B <>剧")
	for _, want := range []string{`SetAVTransportURI`, `http://a/b.m3u8`, `A&amp;B &lt;&gt;剧`, `InstanceID>0<`} {
		if !strings.Contains(body, want) {
			t.Errorf("SOAP body 缺少 %q:\n%s", want, body)
		}
	}
}

func TestCastDeviceDescriptionParse(t *testing.T) {
	XML := `<root xmlns="urn:schemas-upnp-org:device-1-0">
	<device>
		<friendlyName>客厅电视</friendlyName>
		<serviceList>
			<service>
				<serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType>
				<controlURL>/ctl/avt</controlURL>
			</service>
		</serviceList>
	</device>
	</root>`
	server := NewCastDescriptionServerForTest(t, XML)
	defer server.Close()
	device := castDevice{ID: "u1", Location: server.URL + "/dd.xml", Server: "test"}
	parsed, err := loadCastDeviceDescription(context.Background(), device)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Name != "客厅电视" {
		t.Errorf("name = %s", parsed.Name)
	}
	if parsed.ControlURL != server.URL+"/ctl/avt" {
		t.Errorf("controlURL = %s", parsed.ControlURL)
	}
}

// 回归：Dart 端 castStart 只带 playbackSession（无 Drama），授权必须走活动会话
// 而不是 Drama 校验——否则每次投屏都误报「当前版本不包含此站源」。
func TestCastStartAuthorizeBySession(t *testing.T) {
	input := nativeInput{Action: "castStart", MediaURL: "https://x/v.m3u8"}
	// 场景1：无会话、无 Drama -> 应明确报源错误（未被会话分支吃掉）。
	if err := nativeAuthorizeInput(input); err != errNativeBuildSource {
		t.Errorf("空 Drama 且无会话: got %v, want errNativeBuildSource", err)
	}
	// 场景2：playbackSession 存在时 nativeAuthorizeInput 放行（会话真伪由 castStart 内部查表）。
	input.PlaybackSession = "sess-x"
	if err := nativeAuthorizeInput(input); err != nil {
		t.Errorf("带会话应放行授权, got %v", err)
	}
}

// 回归：加密流 / 带 Referer 的计划不允许投屏（电视解不了码、源站拒电视直连）。
func TestCastBlockReason(t *testing.T) {
	if got := castBlockReason(nil); got != "" {
		t.Errorf("空媒体应可投, got %q", got)
	}
	if got := castBlockReason([]providerMedia{{URL: "http://a/v.mp4"}}); got != "" {
		t.Errorf("普通流应可投, got %q", got)
	}
	if got := castBlockReason([]providerMedia{{URL: "http://a/v", CENCKey: []byte{1}}}); got == "" {
		t.Error("CENC 加密流应被拦截")
	}
	if got := castBlockReason([]providerMedia{{URL: "http://a/v", HLSKey: []byte{1}}}); got == "" {
		t.Error("HLS 加密流应被拦截")
	}
	if got := castBlockReason([]providerMedia{{URL: "http://a/v", Referer: "http://src/"}}); got == "" {
		t.Error("带 Referer 的流应被拦截")
	}
	// 多线路: 任一路需 Referer 即拦截。
	if got := castBlockReason([]providerMedia{{URL: "http://a/v"}, {URL: "http://b/v", Referer: "http://src/"}}); got == "" {
		t.Error("多线路中带 Referer 的应被拦截")
	}
}

// 中继单元：m3u8 改写（剥 KEY 行、segment 重写）与 AES-128 分段解密。
func TestRelayPlaylistRewrite(t *testing.T) {
	var upstream string
	mux := http.NewServeMux()
	mux.HandleFunc("/v/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		// 验证中继补头：带 Referer 才给内容。
		if r.Header.Get("Referer") == "" {
			http.Error(w, "need referer", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n" +
			"#EXT-X-VERSION:3\n" +
			"#EXT-X-TARGETDURATION:4\n" +
			"#EXT-X-KEY:METHOD=AES-128,URI=\"https://k.example/key\",IV=0x00112233445566778899aabbccddeeff\n" +
			"#EXTINF:4.0,\n" +
			"https://cdn.example/seg-0.ts\n" +
			"#EXTINF:4.0,\n" +
			"/rel/seg-1.ts\n"))
	})
	mux.HandleFunc("/rel/seg-1.ts", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("x")) })
	server := &http.Server{Handler: mux}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	upstream = "http://" + listener.Addr().String() + "/v/index.m3u8"

	stream := &relayStream{key: "t1", playlistURL: upstream, referer: "https://src.example/"}
	relayMu.Lock()
	relayStreams["t1"] = stream
	relayMu.Unlock()
	defer func() {
		relayMu.Lock()
		delete(relayStreams, "t1")
		relayMu.Unlock()
	}()

	recorder := httptestNewRecorder()
	relayHandler(recorder, newRelayRequest("/relay/t1/index.m3u8"))
	code, _ := relayRecorderResult(recorder)
	if code != http.StatusOK {
		t.Fatalf("playlist HTTP %d", code)
	}
	text := recorder.Body.String()
	if strings.Contains(text, "EXT-X-KEY") {
		t.Error("KEY 行未剥离")
	}
	if !strings.Contains(text, "/relay/t1/seg?n=0") || !strings.Contains(text, "/relay/t1/seg?n=1") {
		t.Errorf("segment 未重写:\n%s", text)
	}
	if !strings.Contains(text, "cdn.example") && strings.Contains(text, "https://cdn.example/seg-0.ts") {
		t.Error("绝对地址应被重写")
	}
	stream.mu.Lock()
	segs, iv := stream.segments, stream.iv
	stream.mu.Unlock()
	if len(segs) != 2 {
		t.Fatalf("segments = %d", len(segs))
	}
	if segs[0] != "https://cdn.example/seg-0.ts" {
		t.Errorf("seg0 = %s", segs[0])
	}
	wantIV := []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	if string(iv) != string(wantIV) {
		t.Errorf("IV = %x", iv)
	}
}

func TestRelaySegmentDecrypt(t *testing.T) {
	// AES-128-CBC: 加密 "0123456789abcdef" 16字节明文。
	key := []byte("0123456789abcdef")
	iv := []byte("fedcba9876543210")
	block, _ := aes.NewCipher(key)
	plain := []byte("relay-test-data!")
	padded := append(plain, make([]byte, 16-len(plain)%16)...)
	cipherText := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(cipherText, padded)

	got, err := aes128Decrypt(cipherText, key, iv)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(got), "relay-test-data") {
		t.Errorf("解密结果 %q", got)
	}
	// 序号 IV。
	sequenceIV(5)
}

func TestParseHLSIV(t *testing.T) {
	iv := parseHLSIV(`#EXT-X-KEY:METHOD=AES-128,URI="https://k/x",IV=0xdeadbeef`)
	want := []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0xde, 0xad, 0xbe, 0xef}
	if string(iv) != string(want) {
		t.Errorf("IV = %x", iv)
	}
	if parseHLSIV("#EXT-X-KEY:METHOD=AES-128,URI=\"https://k/x\"") != nil {
		t.Error("无 IV 应返回 nil")
	}
}

// 端到端：模拟「加密+需 Referer」的上游（红果形态）——castStart 在会话内
// 应转中继；直接走 relayPlaylist/relaySegment 全链路取回解密后的明文 TS。
func TestRelayEndToEndEncrypted(t *testing.T) {
	key := []byte("0123456789abcdef")
	iv := sequenceIV(0) // 上游未给 IV → 按规范用媒体序号
	plainSeg := bytes.Repeat([]byte{0x47}, 32) // 模拟 TS 同步字

	block, _ := aes.NewCipher(key)
	encrypted := make([]byte, len(plainSeg))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(encrypted, plainSeg)

	mux := http.NewServeMux()
	mux.HandleFunc("/v/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://src.example/" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte("#EXTM3U\n#EXT-X-KEY:METHOD=AES-128,URI=\"https://k/k\",\n" +
			"#EXTINF:4.0,\n" + "http://"+r.Host+"/v/s0.ts\n"))
	})
	mux.HandleFunc("/v/s0.ts", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Referer") != "https://src.example/" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		_, _ = w.Write(encrypted)
	})
	listener, _ := net.Listen("tcp", "127.0.0.1:0")
	server := &http.Server{Handler: mux}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })

	stream := &relayStream{
		key:         "e2e",
		playlistURL: "http://" + listener.Addr().String() + "/v/index.m3u8",
		referer:     "https://src.example/",
		aesKey:      key,
	}
	relayMu.Lock()
	relayStreams["e2e"] = stream
	relayMu.Unlock()
	defer func() {
		relayMu.Lock()
		delete(relayStreams, "e2e")
		relayMu.Unlock()
	}()

	// 1) 改写后的播放列表不应再含 KEY 行。
	rec := httptestNewRecorder()
	relayHandler(rec, newRelayRequest("/relay/e2e/index.m3u8"))
	if rec.Code != http.StatusOK {
		t.Fatalf("playl ist HTTP %d", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "EXT-X-KEY") {
		t.Fatal("KEY 未剥离")
	}
	// 2) 拉分段 → 得到明文。
	stream.mu.Lock()
	_ = stream.segments
	stream.mu.Unlock()
	rec2 := httptestNewRecorder()
	relayHandler(rec2, newRelayRequest("/relay/e2e/seg?n=0"))
	if rec2.Code != http.StatusOK {
		t.Fatalf("seg HTTP %d body=%s", rec2.Code, rec2.Body.String())
	}
	if !bytes.Equal(rec2.Body.Bytes(), plainSeg) {
		t.Errorf("解密输出不等于明文: %x", rec2.Body.Bytes())
	}
}
