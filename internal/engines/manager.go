package engines

import (
	"archive/tar"
	"archive/zip"
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

	"github.com/ulikunitz/xz"
)

// httpClient is shared across all manager operations.
// 30s timeout is generous enough for large binary downloads on slow connections.
var httpClient = &http.Client{Timeout: 30 * time.Minute}

// headClient is used only for quick metadata fetches — short timeout.
var headClient = &http.Client{Timeout: 15 * time.Second}

// archiveBinaries maps engine names to the binary filenames to extract from
// their release archives. ffmpeg ships both ffmpeg and ffprobe in the same
// archive — we need both.
var archiveBinaries = map[string][]string{
	EngineFFmpeg: {"ffmpeg", "ffprobe"},
}

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

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("could not create bin directory: %w", err)
	}

	tmpPath := filepath.Join(binDir, "."+engineName+".tmp")
	defer os.Remove(tmpPath)

	if err := downloadFile(downloadURL, tmpPath, progress); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Verify SHA256 against the release's checksum file (yt-dlp only).
	// For gallery-dl and ffmpeg there's no published hash file — we rely on
	// HTTPS + GitHub's integrity.
	if engineName == EngineYtDlp {
		if err := verifyYtDlpChecksum(release, assetName, tmpPath); err != nil {
			return fmt.Errorf("checksum verification failed: %w", err)
		}
	}

	// If this engine ships as an archive, extract the binaries from it.
	// Otherwise treat the downloaded file as the binary itself.
	if binNames, isArchive := archiveBinaries[engineName]; isArchive {
		if err := extractArchive(tmpPath, assetName, binDir, binNames, goos); err != nil {
			return fmt.Errorf("extraction failed: %w", err)
		}
		// tmpPath is the archive — defer already removes it.
		return nil
	}

	// Single-binary path (yt-dlp, gallery-dl): move temp file into place.
	finalPath := filepath.Join(binDir, binaryFilename(engineName))
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return fmt.Errorf("could not install binary: %w", err)
	}
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

// ---- Archive extraction --------------------------------------------------

// extractArchive unpacks binNames out of the archive at archivePath into binDir.
// Supports .tar.xz (linux ffmpeg) and .zip (windows ffmpeg).
func extractArchive(archivePath, assetName, binDir string, binNames []string, goos string) error {
	switch {
	case strings.HasSuffix(assetName, ".tar.xz"):
		return extractTarXz(archivePath, binDir, binNames, goos)
	case strings.HasSuffix(assetName, ".zip"):
		return extractZip(archivePath, binDir, binNames, goos)
	default:
		return fmt.Errorf("unsupported archive format: %s", assetName)
	}
}

// extractTarXz extracts binNames from a .tar.xz archive into binDir.
// The BtbN ffmpeg archive layout is:
//
//	ffmpeg-master-latest-linux64-gpl/bin/ffmpeg
//	ffmpeg-master-latest-linux64-gpl/bin/ffprobe
//
// We match only on the base filename, so the directory structure doesn't matter.
func extractTarXz(archivePath, binDir string, binNames []string, goos string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("could not open archive: %w", err)
	}
	defer f.Close()

	xzReader, err := xz.NewReader(f)
	if err != nil {
		return fmt.Errorf("could not read xz stream: %w", err)
	}

	return extractFromTar(tar.NewReader(xzReader), binDir, binNames, goos)
}

// extractZip extracts binNames from a .zip archive into binDir.
func extractZip(archivePath, binDir string, binNames []string, goos string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return fmt.Errorf("could not open zip archive: %w", err)
	}
	defer r.Close()

	want := makeWantSet(binNames, goos)
	found := make(map[string]bool)

	for _, f := range r.File {
		base := filepath.Base(f.Name)
		targetName, ok := want[base]
		if !ok {
			continue
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("could not open zip entry %s: %w", f.Name, err)
		}

		if err := writeFile(rc, filepath.Join(binDir, targetName)); err != nil {
			rc.Close()
			return fmt.Errorf("could not write %s: %w", targetName, err)
		}
		rc.Close()
		found[targetName] = true
	}

	return checkAllFound(want, found)
}

// extractFromTar walks a tar stream and writes any entry whose base name is in
// binNames into binDir.
func extractFromTar(tr *tar.Reader, binDir string, binNames []string, goos string) error {
	want := makeWantSet(binNames, goos)
	found := make(map[string]bool)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		base := filepath.Base(hdr.Name)
		targetName, ok := want[base]
		if !ok {
			continue
		}

		if err := writeFile(tr, filepath.Join(binDir, targetName)); err != nil {
			return fmt.Errorf("could not write %s: %w", targetName, err)
		}
		found[targetName] = true
	}

	return checkAllFound(want, found)
}

// makeWantSet builds { archiveBasename -> destFilename } for the binaries we
// want. On Windows the basenames carry a .exe suffix.
func makeWantSet(binNames []string, goos string) map[string]string {
	want := make(map[string]string, len(binNames))
	for _, name := range binNames {
		archiveName := name
		destName := name
		if goos == "windows" {
			archiveName = name + ".exe"
			destName = name + ".exe"
		}
		want[archiveName] = destName
	}
	return want
}

// writeFile streams src into a new file at destPath and marks it executable.
func writeFile(src io.Reader, destPath string) error {
	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, src); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Chmod(destPath, 0755)
}

// checkAllFound returns an error listing binaries not found in the archive.
func checkAllFound(want map[string]string, found map[string]bool) error {
	var missing []string
	for _, dest := range want {
		if !found[dest] {
			missing = append(missing, dest)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("archive did not contain: %s", strings.Join(missing, ", "))
	}
	return nil
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

	total := resp.ContentLength
	var done int64
	buf := make([]byte, 32*1024)

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

func verifyYtDlpChecksum(release *githubRelease, assetName, filePath string) error {
	checksumAssetName := "SHA2-256SUMS"
	checksumURL := ""
	for _, a := range release.Assets {
		if a.Name == checksumAssetName {
			checksumURL = a.BrowserDownloadURL
			break
		}
	}
	if checksumURL == "" {
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

	expectedHash := ""
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == assetName {
			expectedHash = strings.ToLower(parts[0])
			break
		}
	}
	if expectedHash == "" {
		return nil
	}

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
	out, err := runVersionCommand(binaryPath)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// ---- Filesystem helpers --------------------------------------------------

func binaryFilename(engineName string) string {
	goos, _ := CurrentPlatform()
	if goos == "windows" {
		return engineName + ".exe"
	}
	return engineName
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	goos, _ := CurrentPlatform()
	if goos == "windows" {
		return !info.IsDir()
	}
	return info.Mode()&0111 != 0
}

func findInPath(engineName string) string {
	pathEnv := os.Getenv("PATH")
	for _, dir := range filepath.SplitList(pathEnv) {
		candidate := filepath.Join(dir, engineName)
		if isExecutable(candidate) {
			return candidate
		}
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
