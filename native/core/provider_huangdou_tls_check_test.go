package core

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// 黄豆播放链路的证书宽松标记必须生效：严格客户端拒绝自签名证书，
// 带 providerMediaLenientContext 的请求则改用宽松客户端成功取回播放列表。
func TestMediaRequestLenientTLS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		_, _ = io.WriteString(writer, "#EXTM3U\n#EXTINF:1.0,\nseg-1.ts\n")
	}))
	defer server.Close()

	d := &Downloader{cfg: defaultConfig(), proxyRouter: &proxyRouter{}}

	strictRequest, err := http.NewRequest(http.MethodGet, server.URL+"/api/drama/hls/x/1/play.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	strictRequest.Header.Set("Referer", server.URL+"/home")
	if _, err := d.doMediaRequestWithClient(strictRequest, &http.Client{}); err == nil {
		t.Fatal("严格校验不应接受自签名证书")
	}

	looseRequest, err := http.NewRequest(http.MethodGet, server.URL+"/api/drama/hls/x/1/play.m3u8", nil)
	if err != nil {
		t.Fatal(err)
	}
	looseRequest.Header.Set("Referer", server.URL+"/home")
	looseRequest = looseRequest.WithContext(providerMediaLenientContext(looseRequest.Context()))
	response, err := d.doMediaRequestWithClient(looseRequest, &http.Client{})
	if err != nil {
		t.Fatalf("宽松标记下应能取回播放列表: %v", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "#EXTM3U") {
		t.Fatalf("播放列表内容异常: %q", string(body))
	}
}
