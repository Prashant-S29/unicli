// Copyright © 2026 Prashant Singh
package engines

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	mgr "github.com/prashant-s29/unicli/internal/engines"
)

// ytdlpEngine downloads platform media via the yt-dlp binary.
type ytdlpEngine struct {
	binaryPath string
}

// NewYtDlpEngine resolves the yt-dlp binary and returns a ready engine.
// Returns (nil, "", false, nil) if the binary is not installed.
func NewYtDlpEngine(binDir string) (Engine, string, bool, error) {
	path, managed, err := mgr.Resolve(mgr.EngineYtDlp, binDir)
	if err != nil {
		return nil, "", false, err
	}
	if path == "" {
		return nil, "", false, nil
	}
	return &ytdlpEngine{binaryPath: path}, path, managed, nil
}

func (e *ytdlpEngine) Name() string { return mgr.EngineYtDlp }

// CanHandle just confirms the binary exists.
// We don't run --simulate: it fails on photo-only posts and adds latency.
func (e *ytdlpEngine) CanHandle(_ string) error { return nil }

// Download runs yt-dlp and streams progress via the callback.
//
// yt-dlp stream layout:
//
//	stdout → ALL normal output: [youtube], [download], [info], [ffmpeg] lines
//	stderr → only ERROR: and WARNING: lines
//
// We read stdout for progress and stderr only for error capture.
func (e *ytdlpEngine) Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error {
	if req.DryRun {
		args := append([]string{"--simulate"}, e.buildArgs(req)...)
		cmd := exec.CommandContext(ctx, e.binaryPath, args...)
		out, _ := cmd.CombinedOutput()
		progress(ProgressUpdate{Filename: strings.TrimSpace(string(out)), TotalBytes: -1, Done: true})
		return nil
	}

	args := e.buildArgs(req)
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)

	// stdout → [download] progress lines, [info], [youtube] extraction lines
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not attach to yt-dlp stdout: %w", err)
	}
	// stderr → ERROR:/WARNING: lines only
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not attach to yt-dlp stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start yt-dlp: %w", err)
	}

	// Collect stderr in background for error reporting
	var stderrLines []string
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			stderrLines = append(stderrLines, sc.Text())
		}
	}()

	// Parse progress from stdout
	var currentFilename string
	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := sc.Text()
		update, ok := parseProgressLine(line)
		if !ok {
			continue
		}
		if update.Filename != "" {
			currentFilename = update.Filename
		} else if currentFilename != "" {
			update.Filename = currentFilename
		}
		progress(update)
	}

	<-stderrDone

	if err := cmd.Wait(); err != nil {
		errMsg := extractErrorMessage(stderrLines)
		if errMsg == "" {
			errMsg = err.Error()
		}
		return fmt.Errorf("yt-dlp failed: %s", errMsg)
	}

	// Emit final Done update
	progress(ProgressUpdate{
		Filename: currentFilename,
		Done:     true,
	})

	return nil
}

// parseProgressLine parses yt-dlp stdout lines into ProgressUpdates.
//
// Lines we handle:
//
//	[download] Destination: /path/to/file.mp4
//	[download] /path/to/file.mp4 has already been downloaded
//	[download]  62.3% of   73.14MiB at    3.20MiB/s ETA 00:08
//	[download] 100% of   73.14MiB in 00:23 at    3.20MiB/s
func parseProgressLine(line string) (ProgressUpdate, bool) {
	trimmed := strings.TrimSpace(line)

	// [Merger] line gives us the true final filename after muxing.
	// Must be checked before the [download] prefix check.
	if strings.HasPrefix(trimmed, "[Merger]") {
		// [Merger] Merging formats into "./path/to/file.webm"
		if idx := strings.Index(trimmed, `"`); idx >= 0 {
			rest := trimmed[idx+1:]
			if end := strings.Index(rest, `"`); end >= 0 {
				merged := rest[:end]
				return ProgressUpdate{Filename: filepath.Base(merged)}, true
			}
		}
		return ProgressUpdate{}, false
	}

	// [ExtractAudio] line gives us the final filename for audio-only downloads.
	// [Merger] handles video+audio mux; [ExtractAudio] handles -x / --audio-only.
	if strings.HasPrefix(trimmed, "[ExtractAudio]") {
		// [ExtractAudio] Destination: ./path/to/file.mp3
		if strings.Contains(trimmed, "Destination:") {
			idx := strings.Index(trimmed, "Destination:")
			dest := strings.TrimSpace(trimmed[idx+len("Destination:"):])
			return ProgressUpdate{Filename: filepath.Base(dest)}, true
		}
		return ProgressUpdate{}, false
	}

	if !strings.HasPrefix(trimmed, "[download]") {
		return ProgressUpdate{}, false
	}

	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, "[download]"))

	// Destination: gives us the filename before download starts.
	// Strip .fNNN fragment suffix (e.g. .f400.mp4 → .mp4) for display.
	if strings.HasPrefix(rest, "Destination:") {
		dest := strings.TrimSpace(strings.TrimPrefix(rest, "Destination:"))
		name := filepath.Base(dest)
		name = stripFragmentSuffix(name)
		return ProgressUpdate{Filename: name}, true
	}

	// Already downloaded — treat as instant done for this file
	if strings.Contains(rest, "has already been downloaded") {
		// Extract the path: it's everything before " has already been downloaded"
		path := strings.TrimSpace(strings.Split(rest, " has already been downloaded")[0])
		return ProgressUpdate{
			Filename: filepath.Base(path),
			Done:     true,
		}, true
	}

	// Progress percentage line — must contain "%"
	pctIdx := strings.Index(rest, "%")
	if pctIdx < 0 {
		return ProgressUpdate{}, false
	}

	pctStr := strings.TrimSpace(rest[:pctIdx])
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		return ProgressUpdate{}, false
	}

	update := ProgressUpdate{}

	// Total size: "of X.XXMiB"
	if ofIdx := strings.Index(rest, " of "); ofIdx >= 0 {
		fields := strings.Fields(rest[ofIdx+4:])
		if len(fields) > 0 {
			total := parseSizeString(fields[0])
			update.TotalBytes = total
			if total > 0 && pct > 0 {
				update.DoneBytes = int64(float64(total) * pct / 100.0)
			}
		}
	}

	// Speed: "at X.XXMiB/s"
	if atIdx := strings.Index(rest, " at "); atIdx >= 0 {
		fields := strings.Fields(rest[atIdx+4:])
		if len(fields) > 0 {
			update.Speed = parseSpeedString(strings.TrimSuffix(fields[0], "/s"))
		}
	}

	// ETA: "ETA HH:MM:SS" or "ETA MM:SS"
	if etaIdx := strings.Index(rest, "ETA "); etaIdx >= 0 {
		fields := strings.Fields(rest[etaIdx+4:])
		if len(fields) > 0 {
			update.ETA = parseETAString(fields[0])
		}
	}

	return update, true
}

// parseSizeString converts "73.14MiB", "1.23GiB", "456KiB" etc. to bytes.
func parseSizeString(s string) int64 {
	units := []struct {
		suffix string
		mult   float64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3}, {"B", 1},
	}
	for _, u := range units {
		if strings.HasSuffix(s, u.suffix) {
			val, err := strconv.ParseFloat(strings.TrimSuffix(s, u.suffix), 64)
			if err == nil {
				return int64(val * u.mult)
			}
		}
	}
	return 0
}

func parseSpeedString(s string) float64 { return float64(parseSizeString(s)) }

// parseETAString converts "HH:MM:SS" or "MM:SS" to a Duration.
func parseETAString(s string) time.Duration {
	if s == "" || s == "Unknown" || s == "--:--" {
		return 0
	}
	parts := strings.Split(s, ":")
	var seconds int64
	for _, p := range parts {
		v, err := strconv.ParseInt(p, 10, 64)
		if err != nil {
			return 0
		}
		seconds = seconds*60 + v
	}
	return time.Duration(seconds) * time.Second
}

// extractErrorMessage finds the last ERROR: line from yt-dlp stderr
// and translates known technical errors into human-readable messages.
func extractErrorMessage(lines []string) string {
	raw := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "ERROR:") {
			raw = strings.TrimSpace(strings.TrimPrefix(line, "ERROR:"))
			break
		}
	}
	if raw == "" {
		for i := len(lines) - 1; i >= 0; i-- {
			if line := strings.TrimSpace(lines[i]); line != "" {
				raw = line
				break
			}
		}
	}

	// Translate known yt-dlp errors into actionable messages
	switch {
	case strings.Contains(raw, "No video could be found in this tweet"):
		return "this tweet contains only images — image downloading from Twitter comes in the next release"
	case strings.Contains(raw, "Unable to extract video") && strings.Contains(raw, "LinkedIn"):
		return "this LinkedIn post contains only images, which aren't supported yet — only video posts can be downloaded"
	case strings.Contains(raw, "Unsupported URL"):
		return "could not find downloadable media at this URL"
	}

	return raw
}

// buildArgs constructs the yt-dlp argument list from a DownloadRequest.
func (e *ytdlpEngine) buildArgs(req DownloadRequest) []string {
	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = "."
	}

	args := []string{
		"--newline",
		"--progress",
		"--no-warnings",
		"--no-playlist",
		"--ignore-no-formats-error",
		"-o", outputDir + "/unicli_%(epoch)s_%(title)s.%(ext)s",
	}
	switch {
	case req.AudioOnly:
		audioFmt := "mp3"
		if req.Format != "" {
			audioFmt = req.Format
		}
		args = append(args, "-x", "--audio-format", audioFmt)

	case req.Format != "" && req.Quality != "" && req.Quality != "best":
		height := strings.TrimSuffix(req.Quality, "p")
		args = append(args,
			"-f", fmt.Sprintf(
				"bestvideo[height<=%s][ext=%s]+bestaudio/bestvideo[height<=%s]+bestaudio/best",
				height, req.Format, height,
			),
			"--merge-output-format", req.Format,
		)

	case req.Format != "":
		args = append(args,
			"-f", fmt.Sprintf("bestvideo[ext=%s]+bestaudio/bestvideo+bestaudio/best", req.Format),
			"--merge-output-format", req.Format,
		)

	case req.Quality != "" && req.Quality != "best":
		height := strings.TrimSuffix(req.Quality, "p")
		args = append(args,
			"-f", fmt.Sprintf("bestvideo[height<=%s]+bestaudio/best[height<=%s]/best", height, height),
		)

	default:
		args = append(args, "-f", "bestvideo+bestaudio/best")
	}

	if req.NoMeta {
		args = append(args, "--no-embed-metadata")
	}

	args = append(args, req.URL)
	return args
}

// stripFragmentSuffix removes yt-dlp's intermediate format suffix from filenames.
// e.g. "video.f400.mp4" → "video.mp4", "audio.f251.webm" → "audio.webm"
// These fragment files are merged and deleted by yt-dlp — the user never sees them.
func stripFragmentSuffix(name string) string {
	ext := filepath.Ext(name)             // ".mp4"
	stem := strings.TrimSuffix(name, ext) // "video.f400"
	if dotIdx := strings.LastIndex(stem, "."); dotIdx >= 0 {
		maybeFragment := stem[dotIdx+1:] // "f400"
		if len(maybeFragment) > 1 && maybeFragment[0] == 'f' {
			allDigits := true
			for _, c := range maybeFragment[1:] {
				if c < '0' || c > '9' {
					allDigits = false
					break
				}
			}
			if allDigits {
				return stem[:dotIdx] + ext // "video.mp4"
			}
		}
	}
	return name
}
