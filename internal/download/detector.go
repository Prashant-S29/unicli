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
	PlatformGallery     // gallery-dl territory (Pixiv, DeviantArt, etc.)
	PlatformUnsupported // known platform, no engine supports it
	PlatformUnknown     // fall through to yt-dlp best-effort
)

// DetectResult is what the detector returns to the orchestrator.
type DetectResult struct {
	Platform        Platform
	RecommendEngine string // engine name constant from internal/engines — "" for unsupported
	FallbackEngine  string // tried if primary fails with ErrNoMedia — "" means no fallback
}

// directFileExts is the set of file extensions we treat as plain HTTP downloads.
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

// platformRule maps a compiled regex to a Platform + engines.
type platformRule struct {
	re             *regexp.Regexp
	platform       Platform
	engine         string // primary engine
	fallbackEngine string // "" means no fallback
}

// rules are evaluated top-to-bottom. First match wins.
//
// Detection order:
//  1. Unsupported platforms — fail fast with a clear message
//  2. Gallery-only platforms (gallery-dl) — more specific, checked before yt-dlp
//  3. Hybrid platforms — yt-dlp primary (video), gallery-dl fallback (images)
//  4. Video-only platforms — yt-dlp, no fallback
var rules = []platformRule{

	// ---- Unsupported platforms — fail fast --------------------------------

	// LinkedIn: neither engine supports it
	{
		re:             regexp.MustCompile(`(?i)(linkedin\.com)`),
		platform:       PlatformUnsupported,
		engine:         "",
		fallbackEngine: "",
	},

	// ---- Gallery-only platforms (gallery-dl) ------------------------------

	// Pixiv
	{
		re:             regexp.MustCompile(`(?i)(pixiv\.net)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// DeviantArt
	{
		re:             regexp.MustCompile(`(?i)(deviantart\.com|fav\.me)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Danbooru and its forks
	{
		re:             regexp.MustCompile(`(?i)(danbooru\.donmai\.us|safebooru\.donmai\.us)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Gelbooru
	{
		re:             regexp.MustCompile(`(?i)(gelbooru\.com)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Sankaku Complex
	{
		re:             regexp.MustCompile(`(?i)(sankakucomplex\.com|chan\.sankakucomplex\.com|idol\.sankakucomplex\.com)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// ArtStation
	{
		re:             regexp.MustCompile(`(?i)(artstation\.com)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Flickr
	{
		re:             regexp.MustCompile(`(?i)(flickr\.com|flic\.kr)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Imgur galleries and albums only — single image URLs are direct files
	{
		re:             regexp.MustCompile(`(?i)(imgur\.com/(a|gallery)/)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// Wallhaven
	{
		re:             regexp.MustCompile(`(?i)(wallhaven\.cc)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},
	// e621 / e926
	{
		re:             regexp.MustCompile(`(?i)(e621\.net|e926\.net)`),
		platform:       PlatformGallery,
		engine:         mgr.EngineGalleryDl,
		fallbackEngine: "",
	},

	// ---- Hybrid platforms — yt-dlp for video, gallery-dl fallback for images

	// YouTube — video only, no image content to fall back to
	{
		re:             regexp.MustCompile(`(?i)(youtube\.com|youtu\.be|youtube-nocookie\.com|music\.youtube\.com)`),
		platform:       PlatformYouTube,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: "",
	},
	// Instagram — posts can be video or image(s)
	{
		re:             regexp.MustCompile(`(?i)(instagram\.com|instagr\.am)`),
		platform:       PlatformInstagram,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: mgr.EngineGalleryDl,
	},
	// Twitter / X — tweets can be video, images, or both
	{
		re:             regexp.MustCompile(`(?i)(twitter\.com|x\.com|t\.co)`),
		platform:       PlatformTwitter,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: mgr.EngineGalleryDl,
	},
	// TikTok — video only
	{
		re:             regexp.MustCompile(`(?i)(tiktok\.com|vm\.tiktok\.com)`),
		platform:       PlatformTikTok,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: "",
	},
	// Reddit — posts can be video (v.redd.it) or images (i.redd.it / galleries)
	{
		re:             regexp.MustCompile(`(?i)(reddit\.com|redd\.it|v\.redd\.it|i\.redd\.it)`),
		platform:       PlatformReddit,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: mgr.EngineGalleryDl,
	},
	// Vimeo — video only
	{
		re:             regexp.MustCompile(`(?i)(vimeo\.com|player\.vimeo\.com)`),
		platform:       PlatformVimeo,
		engine:         mgr.EngineYtDlp,
		fallbackEngine: "",
	},
}

// unsupportedMessages maps Platform to a user-facing explanation.
// Only populated for PlatformUnsupported entries.
var unsupportedMessages = map[*regexp.Regexp]string{
	rules[0].re: "LinkedIn downloads are not supported — the platform requires authentication that cannot be automated",
}

// Detect inspects the URL and returns which platform and engines to use.
//
// Detection order:
//  1. Known platform patterns (rules slice above) — unsupported, gallery, hybrid, video
//  2. URL path ends in a known file extension → http engine
//  3. Anything else → PlatformUnknown, yt-dlp best-effort
func Detect(rawURL string) DetectResult {
	for _, rule := range rules {
		if rule.re.MatchString(rawURL) {
			return DetectResult{
				Platform:        rule.platform,
				RecommendEngine: rule.engine,
				FallbackEngine:  rule.fallbackEngine,
			}
		}
	}

	if isDirectFileHost(rawURL) || hasDirectFileExt(rawURL) {
		return DetectResult{
			Platform:        PlatformDirectFile,
			RecommendEngine: mgr.EngineHTTP,
			FallbackEngine:  "",
		}
	}

	return DetectResult{
		Platform:        PlatformUnknown,
		RecommendEngine: mgr.EngineYtDlp,
		FallbackEngine:  "",
	}
}

// UnsupportedMessage returns the human-readable explanation for a URL
// that matched a PlatformUnsupported rule. Returns "" for all other platforms.
func UnsupportedMessage(rawURL string) string {
	for re, msg := range unsupportedMessages {
		if re.MatchString(rawURL) {
			return msg
		}
	}
	return ""
}

// directFileHosts are CDN/image hosts that always serve direct files
// regardless of whether the URL path has a recognisable extension.
var directFileHosts = []string{
	"plus.unsplash.com",
	"images.unsplash.com",
	"pbs.twimg.com",    // Twitter image CDN
	"cdninstagram.com", // Instagram image CDN
	"i.redd.it",        // Reddit image CDN (single images)
	"preview.redd.it",
	"i.imgur.com", // Imgur single images
}

func isDirectFileHost(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	for _, host := range directFileHosts {
		if strings.Contains(lower, host) {
			return true
		}
	}
	return false
}

// hasDirectFileExt returns true if the URL path (minus query/fragment) ends
// in one of the known direct-file extensions.
func hasDirectFileExt(rawURL string) bool {
	if i := strings.Index(rawURL, "#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	if i := strings.Index(rawURL, "?"); i >= 0 {
		rawURL = rawURL[:i]
	}
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		rawURL = rawURL[i:]
	}
	if i := strings.LastIndex(rawURL, "."); i >= 0 {
		ext := strings.ToLower(rawURL[i:])
		return directFileExts[ext]
	}
	return false
}
