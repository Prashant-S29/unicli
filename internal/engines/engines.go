package engines

import (
	"fmt"
	"runtime"
)

// Engine name constants — used everywhere an engine is referenced by name.
const (
	EngineHTTP      = "http"
	EngineYtDlp     = "yt-dlp"
	EngineGalleryDl = "gallery-dl"
	EngineFFmpeg    = "ffmpeg"
)

// Release API base URLs. Both GitHub and Codeberg (Gitea) expose compatible
// release/asset JSON shapes — same tag_name, assets[].name, assets[].browser_download_url.
const (
	githubAPI   = "https://api.github.com/repos"
	codebergAPI = "https://codeberg.org/api/v1/repos"
)

// EngineInfo holds everything needed to download and manage one engine binary.
type EngineInfo struct {
	// Name is the canonical engine identifier (matches the constants above).
	Name string

	// Description is shown to the user during setup.
	Description string

	// RepoOwner / RepoName identify the project on the release host.
	RepoOwner string
	RepoName  string

	// ReleaseAPIBase is the root URL of the Gitea/GitHub-compatible releases API.
	// e.g. "https://api.github.com/repos" or "https://codeberg.org/api/v1/repos"
	ReleaseAPIBase string

	// AssetName returns the correct release asset filename for the current platform.
	// Returns an error if the platform is not supported.
	AssetName func(goos, goarch string) (string, error)
}

// registry is the list of all managed engines, in install order.
var registry = []EngineInfo{
	{
		Name:           EngineYtDlp,
		Description:    "media downloader — YouTube, Instagram, Twitter/X and 1000+ sites",
		RepoOwner:      "yt-dlp",
		RepoName:       "yt-dlp",
		ReleaseAPIBase: githubAPI,
		AssetName:      ytDlpAsset,
	},
	{
		Name:           EngineGalleryDl,
		Description:    "image gallery downloader — Pixiv, DeviantArt, Danbooru and more",
		RepoOwner:      "mikf",
		RepoName:       "gallery-dl",
		ReleaseAPIBase: codebergAPI,
		AssetName:      galleryDlAsset,
	},
	{
		Name:           EngineFFmpeg,
		Description:    "image and video processing — convert, compress, resize and more",
		RepoOwner:      "BtbN",
		RepoName:       "FFmpeg-Builds",
		ReleaseAPIBase: githubAPI,
		AssetName:      ffmpegAsset,
	},
}

// All returns the full list of managed engines, in the order they should be installed.
func All() []EngineInfo {
	return registry
}

// Get returns the EngineInfo for a given engine name, or an error if unknown.
func Get(name string) (EngineInfo, error) {
	for _, e := range registry {
		if e.Name == name {
			return e, nil
		}
	}
	return EngineInfo{}, fmt.Errorf("unknown engine: %q", name)
}

// CurrentPlatform returns the GOOS and GOARCH for the running binary.
// These are the values the AssetName functions receive.
func CurrentPlatform() (goos, goarch string) {
	return runtime.GOOS, runtime.GOARCH
}

// ---- Asset name resolvers ------------------------------------------------

// ytDlpAsset resolves the correct yt-dlp release binary name for the platform.
//
// yt-dlp release assets:
//
//	linux/amd64   -> yt-dlp
//	linux/arm64   -> yt-dlp_linux_aarch64
//	darwin/amd64  -> yt-dlp_macos_legacy   (intel mac)
//	darwin/arm64  -> yt-dlp_macos          (apple silicon)
//	windows/amd64 -> yt-dlp.exe
func ytDlpAsset(goos, goarch string) (string, error) {
	switch {
	case goos == "linux" && goarch == "amd64":
		return "yt-dlp", nil
	case goos == "linux" && goarch == "arm64":
		return "yt-dlp_linux_aarch64", nil
	case goos == "darwin" && goarch == "amd64":
		return "yt-dlp_macos_legacy", nil
	case goos == "darwin" && goarch == "arm64":
		return "yt-dlp_macos", nil
	case goos == "windows" && goarch == "amd64":
		return "yt-dlp.exe", nil
	default:
		return "", fmt.Errorf("yt-dlp: unsupported platform %s/%s", goos, goarch)
	}
}

// galleryDlAsset resolves the correct gallery-dl release binary name for the platform.
//
// gallery-dl release assets (Codeberg, v1.32.0+):
//
//	linux/amd64   -> gallery-dl.bin
//	darwin/amd64  -> gallery-dl.bin   (same binary; Rosetta 2 handles arm64)
//	darwin/arm64  -> gallery-dl.bin   (runs via Rosetta 2)
//	windows/amd64 -> gallery-dl.exe
//
// Note: there is no prebuilt binary for linux/arm64.
func galleryDlAsset(goos, goarch string) (string, error) {
	switch {
	case goos == "linux" && goarch == "amd64":
		return "gallery-dl.bin", nil
	case goos == "darwin":
		// Both amd64 and arm64 macs use the same binary (Rosetta 2 on arm64).
		return "gallery-dl.bin", nil
	case goos == "windows" && goarch == "amd64":
		return "gallery-dl.exe", nil
	case goos == "linux" && goarch == "arm64":
		return "", fmt.Errorf(
			"gallery-dl: no prebuilt binary for linux/arm64 — install via: pip install gallery-dl",
		)
	default:
		return "", fmt.Errorf("gallery-dl: unsupported platform %s/%s", goos, goarch)
	}
}

// ffmpegAsset resolves the correct ffmpeg release binary archive name for the platform.
// We use BtbN/FFmpeg-Builds which provides static builds for all platforms.
//
//	linux/amd64   -> ffmpeg-master-latest-linux64-gpl.tar.xz
//	linux/arm64   -> ffmpeg-master-latest-linuxarm64-gpl.tar.xz
//	darwin/amd64  -> ffmpeg-master-latest-macos64-gpl.zip  (via homebrew fallback)
//	darwin/arm64  -> ffmpeg-master-latest-macos64-gpl.zip  (via homebrew fallback)
//	windows/amd64 -> ffmpeg-master-latest-win64-gpl.zip
//
// Note: macOS builds are not provided by BtbN. On macOS we fall back to
// instructing the user to install via Homebrew. This is handled in manager.go.
func ffmpegAsset(goos, goarch string) (string, error) {
	switch {
	case goos == "linux" && goarch == "amd64":
		return "ffmpeg-master-latest-linux64-gpl.tar.xz", nil
	case goos == "linux" && goarch == "arm64":
		return "ffmpeg-master-latest-linuxarm64-gpl.tar.xz", nil
	case goos == "windows" && goarch == "amd64":
		return "ffmpeg-master-latest-win64-gpl.zip", nil
	case goos == "darwin":
		return "", fmt.Errorf(
			"ffmpeg: no managed binary for macOS — install via: brew install ffmpeg",
		)
	default:
		return "", fmt.Errorf("ffmpeg: unsupported platform %s/%s", goos, goarch)
	}
}
