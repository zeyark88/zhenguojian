package core

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"duanjuapp/native/core/sign6"
)

func newHongguoDeviceID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10)
	}
	return strconv.FormatUint(1_000_000_000_000_000_000+binary.BigEndian.Uint64(random[:])%8_000_000_000_000_000_000, 10)
}

// signHongguoRequest 给红果 App 接口请求加签名（0.7.16 重写）。
//
// 2026-10-22 实测：红果 App 接口从「只判 `X-Argus` 是否存在」（坑 68）
// 升级为**校验真实签名**，且要求 X-Gorgon / X-Khronos / X-Ladon /
// X-Argus / X-Helios / X-Medusa **六个头齐全**，缺任何一个都返回
// 「HTTP 200 + 0 字节」：首页推荐流、剧集详情、播放取流会同时失效，
// 表现为红果用户的首页永远空、开屏页一直转圈。
//
// 六头算法见子包 `sign6`（逐行移植自开源实现，纯计算、无网络）。
// 注意两点：
//   - 签名对 query 字节级敏感，必须先写完最终 query 再签；
//   - 源码实现会避开 `BranchOf == 1` 的时间点（约 1/3 概率），
//     这里通过微调 `_rticket` 复刻，否则偶发空响应。
func signHongguoRequest(request *http.Request, body []byte, now time.Time) {
	khronos := now.Unix()
	ticketBase := now.UnixMilli()
	for offset := 0; offset < 32; offset++ {
		query := request.URL.Query()
		query.Set("ts", strconv.FormatInt(khronos, 10))
		query.Set("_rticket", strconv.FormatInt(ticketBase+int64(offset), 10))
		request.URL.RawQuery = query.Encode()
		if sign6.BranchOf(request.URL.String(), body, khronos) != 1 {
			break
		}
	}
	for key, value := range sign6.Sign(request.URL.String(), body, khronos) {
		request.Header.Set(key, value)
	}
	if body != nil {
		request.Header.Set("X-SS-STUB", fmt.Sprintf("%X", md5.Sum(body)))
	}
}
