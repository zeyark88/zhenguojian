package core

import (
	"strings"
	"testing"
)

// 蓝果源（0.9.0 合入上游）解析回归：仅验证纯解析函数，不发起网络请求。

func TestLanguoSourceIdentity(t *testing.T) {
	if got := canonicalProviderSource("languo"); got != sourceLanguo {
		t.Fatalf("unexpected canonical id: %s", got)
	}
	if got := canonicalProviderSource("languo.ai"); got != sourceLanguo {
		t.Fatalf("unexpected canonical host id: %s", got)
	}
	if got := canonicalProviderSource("蓝果"); got != sourceLanguo {
		t.Fatalf("chinese alias not canonicalized: %s", got)
	}
	if !isHuangguoProviderSource(sourceLanguo) {
		t.Fatal("languo must be a registered provider source")
	}
	if got := providerSourceForURL("https://languo.ai/video/123/"); got != sourceLanguo {
		t.Fatalf("languo media host not recognized: %s", got)
	}
	if got := providerSourceForURL("https://cdn.example.test/hls/a.m3u8"); got != "" {
		t.Fatalf("cdn host must not map to languo: %s", got)
	}
	if got := (&Downloader{cfg: defaultConfig()}).providerBaseURL(sourceLanguo); got != languoBaseURL {
		t.Fatalf("unexpected languo base url: %s", got)
	}
}

func TestParseLanguoCatalogCards(t *testing.T) {
	body := strings.Join([]string{
		`<a href="/video/123/"><strong>测试剧A</strong></a>`,
		`<a href="/video/124/"><strong> 测试剧B </strong></a>`,
		`<a href="/video/123/"><strong>重复卡片</strong></a>`,
		`<a href="/other/page/"><strong>非剧集链接</strong></a>`,
		`ll-pager__step`,
	}, "\n")
	items := parseLanguoCatalogCards(body, "")
	if len(items) != 2 {
		t.Fatalf("expected 2 unique cards, got %d", len(items))
	}
	first := items[0]
	if first.ID != providerDramaID(sourceLanguo, "123") || first.Source != sourceLanguo || first.SourceID != "123" {
		t.Fatalf("first card identity incorrect: %+v", first)
	}
	if first.Title != "测试剧A" || first.Name != "测试剧A" {
		t.Fatalf("first card title incorrect: %q / %q", first.Title, first.Name)
	}
	if items[1].Title != "测试剧B" {
		t.Fatalf("second card title not trimmed: %q", items[1].Title)
	}
	if !strings.Contains(body, "ll-pager") || len(items) == 0 {
		t.Fatal("pager detection prerequisites broken")
	}
}

func TestParseLanguoDetailAndChapters(t *testing.T) {
	body := strings.Join([]string{
		`<title>测试剧C-蓝果短剧</title>`,
		`<a href="/video/77/ep-1/">第1集</a><a href="/video/77/ep-2/">第2集</a>`,
		`全 2 集`,
		`"title":"测试剧C","desc":"这是一部测试剧","cover":"/cover/c.jpg","score":"9.1",`,
		`"tags":"都市,情感","episode":"更新至 2 集",`,
		`"epPlaySrcs":{"1":"https://cdn.test/a/1.m3u8?auth_key=abc","2":"https://cdn.test/a/2.m3u8?auth_key=def"}`,
	}, "\n")
	drama := parseLanguoDetailMeta(body, "77")
	if drama.Title != "测试剧C" {
		t.Fatalf("detail title incorrect: %q", drama.Title)
	}
	cover, _ := drama.Cover.(string)
	if drama.Desc != "这是一部测试剧" || !strings.HasPrefix(cover, languoBaseURL+"/cover/") {
		t.Fatalf("detail desc/cover not unescaped or absolutized: %q %q", drama.Desc, drama.Cover)
	}
	if drama.Score != "9.1" || len(drama.Tags) != 2 || drama.CategoryName != "都市" {
		t.Fatalf("detail meta incomplete: %+v", drama)
	}
	chapters := parseLanguoChapters(body, "77")
	if len(chapters) != 2 {
		t.Fatalf("expected 2 chapters, got %d", len(chapters))
	}
	if chapters[0].VideoURL == "" || !strings.Contains(chapters[0].VideoURL, "1.m3u8") {
		t.Fatalf("first chapter missing direct media: %+v", chapters[0])
	}
	if chapters[1].VideoURL == "" || !strings.Contains(chapters[1].VideoURL, "2.m3u8") {
		t.Fatalf("second chapter missing direct media: %+v", chapters[1])
	}
	if chapters[1].PageURL != languoBaseURL+"/video/77/ep-2/" {
		t.Fatalf("second chapter page url incorrect: %s", chapters[1].PageURL)
	}
	if !languoIDValid("77") || languoIDValid("abc") || languoIDValid("") || languoIDValid(strings.Repeat("1", 21)) {
		t.Fatal("languo id validation changed")
	}
}
