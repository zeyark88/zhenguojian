package core

import "errors"

var buildAllSources = "false"

var errNativeBuildSource = errors.New("当前版本不包含此站源")

func nativeSourceAvailable(source string) bool {
	source = canonicalProviderSource(source)
	if buildAllSources == "true" {
		return isHuangguoProviderSource(source)
	}
	// 默认版：红果 + 韩小圈 + 鬼片 + 青空（常用四源直显，与 Dart primaryValues 一致）。
	switch source {
	case sourceHongguo, sourceHanxiaoquan, sourceGuipian:
		return true
	}
	return false
}

func nativeDramaAvailable(drama nativeDrama) bool {
	source, _, valid := splitProviderDramaID(drama.ID)
	return valid && nativeSourceAvailable(source) &&
		(drama.Source == "" || canonicalProviderSource(drama.Source) == source)
}

func nativeChapterAvailable(drama nativeDrama, chapter Chapter) bool {
	if !nativeDramaAvailable(drama) {
		return false
	}
	source, _, _ := splitProviderDramaID(drama.ID)
	if chapter.Source != "" && canonicalProviderSource(chapter.Source) != source {
		return false
	}
	if chapterSource, _, valid := splitProviderDramaID(chapter.ID); valid && chapterSource != source {
		return false
	}
	return true
}

func nativeDownloadAvailable(job nativeDownloadJob) bool {
	return nativeChapterAvailable(job.Drama, job.Chapter)
}

func nativeAuthorizeInput(input nativeInput) error {
	switch input.Action {
	case "castDiscover":
		// 局域网设备发现，不涉及站源内容。
	case "castStart":
		// 投屏授权：投屏地址必须来自本机持有的活动播放计划（playbackSession），
		// 或调用方带完整 Drama/Chapter（桌面等场景）。Dart 端只传 playbackSession，
		// 误走 Drama 校验会让每次投屏都报「当前版本不包含此站源」。
		if input.PlaybackSession == "" &&
			(!nativeDramaAvailable(input.Drama) || !nativeChapterAvailable(input.Drama, input.Chapter)) {
			return errNativeBuildSource
		}
	case "recommendations", "cachedRecommendations", "suggestions", "danmaku":
		if !nativeSourceAvailable(sourceHongguo) {
			return errNativeBuildSource
		}
	case "rankings":
		board, found := findRankingBoard(input.Board)
		if !found || !nativeSourceAvailable(board.Source) {
			return errNativeBuildSource
		}
	case "catalog", "cached", "categories", "sourceStatus", "sourceJob", "cancelSourceJob":
		if !nativeSourceAvailable(input.Source) {
			return errNativeBuildSource
		}
		if input.Action == "sourceJob" && input.Drama.ID != "" &&
			(!nativeDramaAvailable(input.Drama) || sourceFromDramaID(input.Drama.ID) != canonicalProviderSource(input.Source)) {
			return errNativeBuildSource
		}
	case "cover", "prepareCover", "detail", "metadata", "resolve", "preload", "prepareHandoff", "enqueueDownloads", "localPlayback":
		if !nativeDramaAvailable(input.Drama) {
			return errNativeBuildSource
		}
		if (input.Action == "resolve" || input.Action == "preload" || input.Action == "prepareHandoff") && !nativeChapterAvailable(input.Drama, input.Chapter) {
			return errNativeBuildSource
		}
	}
	return nil
}
