package core

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type providerMediaCredentials struct {
	cookie    string
	origin    string
	referer   string
	userAgent string
	expires   time.Time
}

type providerMediaCredentialsKey struct{}

func providerMediaContext(ctx context.Context, credentials *providerMediaCredentials) context.Context {
	return context.WithValue(ctx, providerMediaCredentialsKey{}, credentials)
}

// providerMediaLenientKey 标记「本次媒体请求允许宽松校验证书」。
//
// 只对证书确已过期/被重置、但站点仍在正常服务的老站源（目前仅黄豆）开启，
// 由 providerMedia.LenientTLS 携带，避免影响其他站源的严格校验。
type providerMediaLenientKey struct{}

func providerMediaLenientContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, providerMediaLenientKey{}, true)
}

func providerMediaLenient(ctx context.Context) bool {
	value, _ := ctx.Value(providerMediaLenientKey{}).(bool)
	return value
}

// providerMediaSessionContext 同时携带播放凭证与证书宽松标记，供播放/下载链路使用。
func providerMediaSessionContext(ctx context.Context, credentials *providerMediaCredentials, lenient bool) context.Context {
	ctx = providerMediaContext(ctx, credentials)
	if lenient {
		ctx = providerMediaLenientContext(ctx)
	}
	return ctx
}

func providerMediaOrigin(address *url.URL) string {
	if address == nil {
		return ""
	}
	scheme := strings.ToLower(address.Scheme)
	host := strings.ToLower(address.Host)
	if scheme == "https" && address.Port() == "443" || scheme == "http" && address.Port() == "80" {
		host = strings.TrimSuffix(host, ":"+address.Port())
	}
	return scheme + "://" + host
}

func (credentials *providerMediaCredentials) apply(request *http.Request) error {
	request.Header.Del("Cookie")
	if credentials.userAgent != "" {
		request.Header.Set("User-Agent", credentials.userAgent)
	}
	if credentials.referer != "" {
		request.Header.Set("Referer", credentials.referer)
		if address, err := url.Parse(credentials.referer); err == nil {
			request.Header.Set("Origin", providerMediaOrigin(address))
		}
	}
	if providerMediaOrigin(request.URL) == credentials.origin {
		if !credentials.expires.IsZero() && !time.Now().Before(credentials.expires) {
			return errors.New("播放凭证已过期，请重新解析播放")
		}
		request.Header.Set("Cookie", credentials.cookie)
	}
	return nil
}

func (credentials *providerMediaCredentials) client(client *http.Client) *http.Client {
	scoped := *client
	scoped.Jar = nil
	previousRedirect := client.CheckRedirect
	scoped.CheckRedirect = func(request *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return errors.New("媒体重定向过多")
		}
		if previousRedirect != nil {
			if err := previousRedirect(request, via); err != nil {
				return err
			}
		}
		return credentials.apply(request)
	}
	return &scoped
}
