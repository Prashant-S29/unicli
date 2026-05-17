// Copyright © 2026 Prashant Singh
package engines

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// httpClient is shared across all manager operations.
// 30s timeout is generous enough for large binary downloads on slow connections.
var httpClient = &http.Client{Timeout: 30 * time.Minute}

// headClient is used only for quick metadata fetches — short timeout.
var headClient = &http.Client{Timeout: 15 * time.Second}

// ---- Public API ----------------------------------------------------------

// Resolve returns the path to the engine binary.
// Priority: managed bin dir → $PATH.
// Returns ("", false, nil) if the binary is not found anywhere (caller decides
// whether to prompt for install).
func Resolve(engineName, binDir string) (path string, managed bool, err error) {
	// 1. Managed binary takes precedence
	managedPath := filepath.Join(binDir, binaryFilename(engineName))
	if isExecutable(managedPath) {
		return managedPath, true, nil
	}

	// 2. Fall back to $PATH
	if systemPath := findInPath(engineName); systemPath != "" {
		return systemPath, false, nil
	}

	return "", false, nil
}

// DownloadProgress is called repeatedly during a binary download.
// done / total are byte counts; total may be -1 if Content-Length is unknown.
type DownloadProgress func(done, total int64)

// Install fetches the latest release of engineName from GitHub, verifies its
// checksum, and stores it in binDir. progress is called during the download;
// pass nil to silence progress updates.
func Install(engineName, binDir string, progress DownloadProgress) error {
	meta, err := Get(engineName)
	if err != nil {
		return err
	}

	goos, goarch := CurrentPlatform()

	assetName, err := meta.AssetName(goos, goarch)
	if err != nil {
		return err
	}

	// Fetch the latest release metadata from GitHub API
	release, err := fetchLatestRelease(meta.RepoOwner, meta.RepoName, meta.ReleaseAPIBase)
	if err != nil {
		return fmt.Errorf("could not fetch latest release for %s: %w", engineName, err)
	}

	// Find the download URL for our platform asset
	downloadURL, err := findAssetURL(release, assetName)
	if err != nil {
		return fmt.Errorf("%s: asset not found in release %s: %w", engineName, release.TagName, err)
	}

	// Download the binary to a temp file first
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("could not create bin directory: %w", err)
	}

	tmpPath := filepath.Join(binDir, "."+engineName+".tmp")
	defer os.Remove(tmpPath) // clean up on any failure path

	if err := downloadFile(downloadURL, tmpPath, progress); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify SHA256 against the release's checksum file (if available).
	// For yt-dlp this is SHA2-256SUMS; for gallery-dl there's no published hash
	// file — we skip verification and rely on HTTPS + GitHub's integrity.
	if engineName == EngineYtDlp {
		if err := verifyYtDlpChecksum(release, assetName, tmpPath); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// Move temp file to final destination
	finalPath := filepath.Join(binDir, binaryFilename(engineName))
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("could not install binary: %w", err)
	}

	// Mark executable
	if err := os.Chmod(finalPath, 0755); err != nil {
		return fmt.Errorf("could not set executable bit: %w", err)
	}

	return nil
}

// InstalledVersion returns the version string reported by the managed binary,
// or ("", nil) if the binary is not installed.
func InstalledVersion(engineName, binDir string) (string, error) {
	path, managed, err := Resolve(engineName, binDir)
	if err != nil {
		return "", err
	}
	if path == "" || !managed {
		return "", nil
	}

	return readVersion(engineName, path)
}

// LatestVersion fetches the latest release tag from GitHub without downloading anything.
func LatestVersion(engineName string) (string, error) {
	meta, err := Get(engineName)
	if err != nil {
		return "", err
	}

	release, err := fetchLatestRelease(meta.RepoOwner, meta.RepoName, meta.ReleaseAPIBase)
	if err != nil {
		return "", err
	}

	return release.TagName, nil
}

// ---- GitHub release helpers ----------------------------------------------

type githubRelease struct {
	TagName string        `json:"tag_name"`
	Assets  []githubAsset `json:"assets"`
}

type githubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

func fetchLatestRelease(owner, repo, apiBase string) (*githubRelease, error) {
	url := fmt.Sprintf("%s/%s/%s/releases/latest", apiBase, owner, repo)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "unicli")

	resp, err := headClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("network error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, fmt.Errorf("could not parse release JSON: %w", err)
	}

	return &release, nil
}

func findAssetURL(release *githubRelease, assetName string) (string, error) {
	for _, a := range release.Assets {
		if a.Name == assetName {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("no asset named %q in release %s", assetName, release.TagName)
}

// ---- Download + checksum -------------------------------------------------

func downloadFile(url, destPath string, progress DownloadProgress) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("could not create file: %w", err)
	}
	defer out.Close()

	total := resp.ContentLength // -1 if unknown
	var done int64
	buf := make([]byte, 32*1024) // 32 KB chunks

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}
			done += int64(n)
			if progress != nil {
				progress(done, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	return nil
}

// verifyYtDlpChecksum fetches the SHA2-256SUMS file from the same release
// and verifies that our downloaded binary matches.
func verifyYtDlpChecksum(release *githubRelease, assetName, filePath string) error {
	// Find the checksum asset
	checksumAssetName := "SHA2-256SUMS"
	checksumURL := ""
	for _, a := range release.Assets {
		if a.Name == checksumAssetName {
			checksumURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
		// No checksum file in this release — skip silently
		return nil
	}

	resp, err := headClient.Get(checksumURL)
	if err != nil {
		return fmt.Errorf("could not fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read checksums: %w", err)
	}

	// Format: "<hash>  <filename>" one per line
	expectedHash := ""
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expectedHash = strings.ToLower(parts[0])
			break
		}
	}
	if expectedHash == "" {
		// Asset not listed in checksum file — skip
		return nil
	}

	// Hash the downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("could not open file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("could not hash file: %w", err)
	}
	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch — expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

// ---- Version reading -----------------------------------------------------

func readVersion(_ string, binaryPath string) (string, error) {
	// Both yt-dlp and gallery-dl support --version.
	// runVersionCommand lives in exec.go to keep os/exec out of this file.
	out, err := runVersionCommand(binaryPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ---- Filesystem helpers --------------------------------------------------

// binaryFilename returns the filename used in the managed bin dir.
// On Windows yt-dlp ships as .exe; we preserve that.
func binaryFilename(engineName string) string {
	goos, _ := CurrentPlatform()
	if goos == "windows" {
		return engineName + ".exe"
	}
	return engineName
}

// isExecutable returns true if the path exists and is executable.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	// On Windows, any file that exists is considered executable.
	goos, _ := CurrentPlatform()
	if goos == "windows" {
		return !info.IsDir()
	}
	return info.Mode()&0111 != 0
}

// findInPath searches the directories in $PATH for the engine binary.
func findInPath(engineName string) string {
	// Use os.LookPath equivalent manually to keep import footprint low
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, engineName)
		if isExecutable(candidate) {
			return candidate
		}
		// Windows: also try .exe
		goos, _ := CurrentPlatform()
		if goos == "windows" {
			candidateExe := candidate + ".exe"
			if isExecutable(candidateExe) {
				return candidateExe
			}
		}
	}
	return ""
}
