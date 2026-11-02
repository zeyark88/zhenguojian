package core

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	crand "crypto/rand"
)

// 投屏中继：电视拉不了两类流——带 Referer 校验的、AES-128 加密的 HLS。
// 本机起 HTTP 服务做转换：电视→拉手机，手机→补头/解密→源站。
//
//	/relay/{key}/index.m3u8   拉原播放列表并改写(去 KEY 行、segment 指回本机)
//	/relay/{key}/seg?n=序号   拉原分段，按需解密后回给电视
//
// 生命周期：一次投屏一个 key，闲置 15 分钟由 GC 清理。

const relayIdleTimeout = 15 * time.Minute

type relayStream struct {
	key         string
	playlistURL string
	referer     string
	userAgent   string
	// AES-128 解密（红果等加密 HLS）；nil = 纯转发（仅需补头的源）。
	aesKey []byte

	mu       sync.Mutex
	segments []string
	iv       []byte
	played   time.Time
}

var (
	relayServeOnce sync.Once
	relayMu        sync.Mutex
	relayStreams   = map[string]*relayStream{}
	relayPort      int
)

// 对外注入的中继地址（容器/模拟器里需用宿主可达 IP）。
var relayExternalAddress string

// startRelayServer 启动局域网中继 HTTP 服务（只启一次）。
func startRelayServer() {
	listener, err := net.Listen("tcp4", ":0")
	if err != nil {
		return
	}
	relayPort = listener.Addr().(*net.TCPAddr).Port
	server := &http.Server{Handler: http.HandlerFunc(relayHandler)}
	go func() { _ = server.Serve(listener) }()
	go relayJanitor()
}

func relayJanitor() {
	for range time.Tick(time.Minute) {
		cutoff := time.Now().Add(-relayIdleTimeout)
		relayMu.Lock()
		for key, stream := range relayStreams {
			stream.mu.Lock()
			idle := stream.played.Before(cutoff)
			stream.mu.Unlock()
			if idle {
				delete(relayStreams, key)
			}
		}
		relayMu.Unlock()
	}
}

// registerRelayStream 登记一条中继流，返回电视可访问的 m3u8 地址。
func registerRelayStream(stream *relayStream) (string, error) {
	relayServeOnce.Do(startRelayServer)
	if relayPort == 0 {
		return "", fmt.Errorf("中继服务启动失败")
	}
	key := randToken()
	stream.key = key
	stream.played = time.Now()
	relayMu.Lock()
	relayStreams[key] = stream
	relayMu.Unlock()
	return fmt.Sprintf("http://%s/relay/%s/index.m3u8", lanAddress(), key), nil
}

func lanAddress() string {
	host := relayExternalAddress
	if host == "" {
		host = "127.0.0.1"
		if addrs, err := net.InterfaceAddrs(); err == nil {
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok || ipNet.IP.IsLoopback() || ipNet.IP.To4() == nil {
					continue
				}
				host = ipNet.IP.String()
			}
		}
	}
	return host + ":" + strconv.Itoa(relayPort)
}

func randToken() string {
	b := make([]byte, 16)
	if _, err := crand.Read(b); err != nil {
		for i := range b {
			b[i] = byte(i*31 + 7)
		}
	}
	return hex.EncodeToString(b)
}

func relayHandler(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 || parts[0] != "relay" {
		http.NotFound(w, r)
		return
	}
	key := parts[1]
	relayMu.Lock()
	stream, ok := relayStreams[key]
	relayMu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	stream.mu.Lock()
	stream.played = time.Now()
	stream.mu.Unlock()
	switch parts[2] {
	case "index.m3u8":
		relayPlaylist(w, r, stream)
	case "seg":
		relaySegment(w, r, stream)
	default:
		http.NotFound(w, r)
	}
}

// fetchUpstream 以流的上游身份（补 Referer/UA）拉资源。
func fetchUpstream(ctx context.Context, address, referer, userAgent string, limit int64) ([]byte, int, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, 0, err
	}
	if referer != "" {
		request.Header.Set("Referer", referer)
	}
	if userAgent != "" {
		request.Header.Set("User-Agent", userAgent)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, 0, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, response.StatusCode, fmt.Errorf("上游 HTTP %d", response.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, limit))
	return body, response.StatusCode, err
}

// relayPlaylist 拉上游 m3u8 并改写：剥 EXT-X-KEY（解密由本机做）、
// segment URL 重写到本机 /relay/{key}/seg。支持二级播放表（master→media）。
func relayPlaylist(w http.ResponseWriter, r *http.Request, stream *relayStream) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	body, _, err := fetchUpstream(ctx, stream.playlistURL, stream.referer, stream.userAgent, 4<<20)
	if err != nil {
		http.Error(w, "上游播放列表不可用", http.StatusBadGateway)
		return
	}
	text := string(body)
	lines := strings.Split(text, "\n")

	var iv []byte
	segments := make([]string, 0, 64)
	out := make([]string, 0, len(lines)+8)
	out = append(out, "#EXTM3U")
	out = append(out, "#EXT-X-PLAYLIST-TYPE:VOD")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || trimmed == "#EXTM3U":
			continue
		case strings.HasPrefix(trimmed, "#EXT-X-KEY"):
			iv = parseHLSIV(trimmed)
			continue
		case strings.HasPrefix(trimmed, "#EXT-X-MAP"):
			// init 段（fMP4）暂不透传：电视走 TS 兼容路线。
			continue
		case strings.HasPrefix(trimmed, "#"):
			out = append(out, trimmed)
		default:
			absolute := absoluteURL(stream.playlistURL, trimmed)
			segments = append(segments, absolute)
			out = append(out, segURL(r, stream.key, len(segments)-1))
		}
	}
	stream.mu.Lock()
	stream.segments = segments
	stream.iv = iv
	stream.mu.Unlock()

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write([]byte(strings.Join(out, "\n") + "\n"))
}

func segURL(r *http.Request, key string, index int) string {
	host := r.Host
	if host == "" {
		host = lanAddress()
	}
	return fmt.Sprintf("http://%s/relay/%s/seg?n=%d", host, key, index)
}

// relaySegment 拉上游分段；加密流按 AES-128-CBC 解密后回明文。
func relaySegment(w http.ResponseWriter, r *http.Request, stream *relayStream) {
	n, err := strconv.Atoi(r.URL.Query().Get("n"))
	if err != nil || n < 0 {
		http.NotFound(w, r)
		return
	}
	stream.mu.Lock()
	if n >= len(stream.segments) {
		stream.mu.Unlock()
		http.NotFound(w, r)
		return
	}
	address := stream.segments[n]
	iv := stream.iv
	aesKey := stream.aesKey
	stream.mu.Unlock()

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	body, _, err := fetchUpstream(ctx, address, stream.referer, stream.userAgent, 64<<20)
	if err != nil {
		http.Error(w, "分段不可用", http.StatusBadGateway)
		return
	}
	if len(aesKey) > 0 {
		segmentIV := iv
		if segmentIV == nil {
			segmentIV = sequenceIV(n)
		}
		plain, err := aes128Decrypt(body, aesKey, segmentIV)
		if err != nil {
			http.Error(w, "分段解密失败", http.StatusBadGateway)
			return
		}
		body = plain
	}
	w.Header().Set("Content-Type", "video/mp2t")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(body)
}

// aes128Decrypt 标准 AES-128-CBC（HLS 加密分段）；尾部残块保留（TS 容错）。
func aes128Decrypt(ciphertext, key, iv []byte) ([]byte, error) {
	if len(key) != 16 {
		return nil, fmt.Errorf("key 长度非 16")
	}
	if len(ciphertext) < 16 || len(ciphertext)%16 != 0 {
		return nil, fmt.Errorf("密文未按 16 字节对齐")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	plain := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv[:16]).CryptBlocks(plain, ciphertext)
	return plain, nil
}

// parseHLSIV 解析 #EXT-X-KEY 行 IV=0x… 属性。
func parseHLSIV(line string) []byte {
	idx := strings.Index(line, "IV=0x")
	if idx < 0 {
		return nil
	}
	hexText := strings.TrimRight(strings.TrimSpace(line[idx+5:]), ",\" \t")
	for len(hexText) < 32 {
		hexText = "0" + hexText
	}
	raw, err := hex.DecodeString(hexText[:32])
	if err != nil || len(raw) != 16 {
		return nil
	}
	return raw
}

// sequenceIV HLS 规范：未给 IV 时用媒体序列号的大端 16 字节。
func sequenceIV(seq int) []byte {
	iv := make([]byte, 16)
	number := new(big.Int).SetInt64(int64(seq))
	bytes := number.Bytes()
	copy(iv[16-len(bytes):], bytes)
	return iv
}

func absoluteURL(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

// relayKeyForMedia 媒体若带 HLS AES key 则取之（红果等）。CENCKey 与 HLSKey
// 在 AES-128 HLS 场景是同一个 key（mpv --decryption-key 即用此值）。
func relayKeyForMedia(m providerMedia) []byte {
	if len(m.CENCKey) == 16 {
		return m.CENCKey
	}
	if len(m.HLSKey) == 16 {
		return m.HLSKey
	}
	return nil
}
