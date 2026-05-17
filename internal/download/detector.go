// Copyright © 2026 Prashant Singh
package download

import mgr "github.com/prashant-s29/unicli/internal/engines"

// Platform identifies what kind of URL we're dealing with.
// M3 only uses PlatformDirectFile. M4 will add all the others.
type Platform int

const (
	PlatformDirectFile Platform = iota // plain HTTP file URL — handled by http engine
	PlatformYouTube                    // M4
	PlatformInstagram                  // M4
	PlatformTwitter                    // M4
	PlatformTikTok                     // M4
	PlatformReddit                     // M4
	PlatformVimeo                      // M4
	PlatformGallery                    // M5 — gallery-dl territory
	PlatformUnknown                    // M4 — fall through to yt-dlp best-effort
)

// DetectResult is what the detector returns to the orchestrator.
type DetectResult struct {
	Platform        Platform
	RecommendEngine string // engine name constant from internal/engines
}

// Detect inspects the URL and returns which platform and engine to use.
//
// M3 stub: always returns DirectFile / HTTP engine.
// M4 will replace this with real regex-based platform detection.
func Detect(url string) DetectResult {
	_ = url // M4 will use this
	return DetectResult{
		Platform:        PlatformDirectFile,
		RecommendEngine: mgr.EngineHTTP,
	}
}
