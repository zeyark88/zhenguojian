package core

import (
	"bytes"
	"net/http"
	"testing"

	fhttp "github.com/bogdanfinn/fhttp"
)

func TestHuangguoBrowserTransportMatchesHongguoAppHost(t *testing.T) {
	transport := &huangguoBrowserTransport{host: "huangguo.video"}

	get, _ := http.NewRequest(http.MethodGet, "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/video_model/v1/", nil)
	if !transport.matches(get) {
		t.Error("红果 App GET 请求应走指纹客户端")
	}
	post, _ := http.NewRequest(http.MethodPost, "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/video_model/v1/", bytes.NewReader([]byte("{}")))
	if !transport.matches(post) {
		t.Error("红果 App POST 请求应走指纹客户端")
	}
	put, _ := http.NewRequest(http.MethodPut, "https://api5-normal-sinfonlineb.fqnovel.com/x", bytes.NewReader([]byte("{}")))
	if transport.matches(put) {
		t.Error("非白名单方法不应走指纹客户端")
	}
	other, _ := http.NewRequest(http.MethodPost, "https://example.com/api", bytes.NewReader([]byte("{}")))
	if transport.matches(other) {
		t.Error("非红果域名不应走指纹客户端")
	}
}

func TestBrowserProfileNameSelectsOkHttpForHongguoHost(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/video_model/v1/", nil)
	if got := browserProfileName(request); got != hongguoAppProfile {
		t.Errorf("红果域名应使用 %s，实际 %s", hongguoAppProfile, got)
	}
	web, _ := http.NewRequest(http.MethodGet, "https://huangguo.video/category/1", nil)
	if got := browserProfileName(web); got != huangguoBrowserProfile {
		t.Errorf("黄果域名应使用 %s，实际 %s", huangguoBrowserProfile, got)
	}
}

func TestPassthroughAppHeadersKeepsSignatureHeaders(t *testing.T) {
	request, _ := http.NewRequest(http.MethodPost, "https://api5-normal-sinfonlineb.fqnovel.com/novel/player/video_model/v1/", bytes.NewReader([]byte("{}")))
	request.Header.Set("User-Agent", hongguoAppUserAgent)
	request.Header.Set("X-Gorgon", "8404401c00")
	request.Header.Set("X-Khronos", "1793018000")
	request.Header.Set("X-SS-STUB", "ABCDEF01")

	headers := passthroughAppHeaders(request)
	if got := headers["user-agent"]; len(got) == 0 || got[0] != hongguoAppUserAgent {
		t.Errorf("App 请求头被改写：%q", got)
	}
	if headers["x-gorgon"][0] != "8404401c00" || headers["x-khronos"][0] != "1793018000" || headers["x-ss-stub"][0] != "ABCDEF01" {
		t.Error("签名头未被原样转发")
	}
	order := headers[fhttp.HeaderOrderKey]
	if len(order) == 0 || order[0] != "user-agent" {
		t.Error("App 头顺序键缺失或顺序异常")
	}
}
