// Copyright © 2026 Prashant Singh
package download

import (
	"regexp"
	"strings"

	mgr "github.com/prashant-s29/unicli/internal/engines"
)

// Platform identifies what kind of URL we're dealing with.
type Platform int

const (
	PlatformDirectFile Platform = iota // plain HTTP file URL — handled by http engine
	PlatformYouTube
	PlatformInstagram
	PlatformTwitter
	PlatformTikTok
	PlatformReddit
	PlatformVimeo
	PlatformGallery // M5 — gallery-dl territory
	PlatformUnknown // fall through to yt-dlp best-effort
)

// DetectResult is what the detector returns to the orchestrator.
type DetectResult struct {
	Platform        Platform
	RecommendEngine string // engine name constant from internal/engines
}

// directFileExt is the set of file extensions we treat as plain HTTP downloads.
// Matched against the URL path (after stripping query/fragment).
var directFileExts = map[string]bool{
	".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true,
	".7z": true, ".rar": true,
	".pdf": true, ".epub": true, ".mobi": true,
	".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
	".webm": true, ".flv": true,
	".mp3": true, ".m4a": true, ".flac": true, ".wav": true, ".ogg": true,
	".aac": true,
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".svg": true,
	".iso": true, ".dmg": true, ".exe": true, ".deb": true, ".rpm": true,
	".apk": true,
}

// platformRule maps a compiled regex to a Platform + engine.
// Rules are checked in order — first match wins.
type platformRule struct {
	re       *regexp.Regexp
	platform Platform
	engine   string
}

// rules are evaluated top-to-bottom. More specific patterns come first.
// Gallery platforms (M5) are intentionally absent here and will be prepended in M5.
var rules = []platformRule{
	// YouTube — covers youtube.com, youtu.be, youtube-nocookie.com, music.youtube.com
	{
		re:       regexp.MustCompile(`(?i)(youtube\.com|youtu\.be|youtube-nocookie\.com|music\.youtube\.com)`),
		platform: PlatformYouTube,
		engine:   mgr.EngineYtDlp,
	},
	// Instagram
	{
		re:       regexp.MustCompile(`(?i)(instagram\.com|instagr\.am)`),
		platform: PlatformInstagram,
		engine:   mgr.EngineYtDlp,
	},
	// Twitter / X
	{
		re:       regexp.MustCompile(`(?i)(twitter\.com|x\.com|t\.co)`),
		platform: PlatformTwitter,
		engine:   mgr.EngineYtDlp,
	},
	// TikTok
	{
		re:       regexp.MustCompile(`(?i)(tiktok\.com|vm\.tiktok\.com)`),
		platform: PlatformTikTok,
		engine:   mgr.EngineYtDlp,
	},
	// Reddit
	{
		re:       regexp.MustCompile(`(?i)(reddit\.com|redd\.it|v\.redd\.it|i\.redd\.it)`),
		platform: PlatformReddit,
		engine:   mgr.EngineYtDlp,
	},
	// Vimeo
	{
		re:       regexp.MustCompile(`(?i)(vimeo\.com|player\.vimeo\.com)`),
		platform: PlatformVimeo,
		engine:   mgr.EngineYtDlp,
	},
}

// Detect inspects the URL and returns which platform and engine to use.
//
// Detection order:
//  1. Known platform patterns (rules slice above) → yt-dlp
//  2. URL path ends in a known file extension → http engine
//  3. Anything else → PlatformUnknown, yt-dlp best-effort
func Detect(rawURL string) DetectResult {
	// 1. Platform pattern matching
	for _, rule := range rules {
		if rule.re.MatchString(rawURL) {
			return DetectResult{
				Platform:        rule.platform,
				RecommendEngine: rule.engine,
			}
		}
	}

	// 2. Direct file by extension
	if hasDirectFileExt(rawURL) {
		return DetectResult{
			Platform:        PlatformDirectFile,
			RecommendEngine: mgr.EngineHTTP,
		}
	}

	// 3. Unknown — try yt-dlp as best-effort
	return DetectResult{
		Platform:        PlatformUnknown,
		RecommendEngine: mgr.EngineYtDlp,
	}
}

// hasDirectFileExt returns true if the URL path (minus query/fragment) ends
// in one of the known direct-file extensions.
func hasDirectFileExt(rawURL string) bool {
	// Strip fragment
	if i := strings.Index(rawURL, "#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	// Strip query string
	if i := strings.Index(rawURL, "?"); i >= 0 {
		rawURL = rawURL[:i]
	}
	// Find last path segment
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		rawURL = rawURL[i:]
	}
	// Find extension
	if i := strings.LastIndex(rawURL, "."); i >= 0 {
		ext := strings.ToLower(rawURL[i:])
		return directFileExts[ext]
	}
	return false
}
