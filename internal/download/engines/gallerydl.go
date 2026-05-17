// Copyright © 2026 Prashant Singh
package engines

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	mgr "github.com/prashant-s29/unicli/internal/engines"
)

// ErrNoMedia is returned by an engine when the URL is valid but contains
// no downloadable media of the type the engine handles.
// The orchestrator uses this sentinel to trigger fallback engine logic.
var ErrNoMedia = errors.New("no downloadable media found")

// galleryDlEngine downloads image galleries via the gallery-dl binary.
//
// Output stream contract (with -o output.mode=pipe):
//
//	stdout → one local file path per downloaded file, newline-terminated
//	         e.g. "./gallery-dl/pixiv/username/12345678_p0.jpg"
//	stderr → all logging: [info], [warning], [error] lines
//	         the per-file byte progress indicator uses \r and is suppressed
//	         via -o output.progress=false so it never appears on stderr
//
// Strategy: track filenames from stdout, emit one ProgressUpdate per file.
// TotalBytes is always -1 (gallery-dl does not expose total gallery size upfront).
// The progress bar renders file count instead of bytes.
type galleryDlEngine struct {
	binaryPath string
}

// NewGalleryDlEngine resolves the gallery-dl binary and returns a ready engine.
// Returns (nil, "", false, nil) if the binary is not installed — same
// contract as NewYtDlpEngine so the orchestrator can handle both uniformly.
func NewGalleryDlEngine(binDir string) (Engine, string, bool, error) {
	path, managed, err := mgr.Resolve(mgr.EngineGalleryDl, binDir)
	if err != nil {
		return nil, "", false, err
	}
	if path == "" {
		return nil, "", false, nil
	}
	return &galleryDlEngine{binaryPath: path}, path, managed, nil
}

func (e *galleryDlEngine) Name() string { return mgr.EngineGalleryDl }

// CanHandle confirms the binary exists.
// No URL pre-flight — gallery-dl surfaces unsupported URLs with a clear
// [error] line on stderr, which Download() translates into ErrNoMedia.
func (e *galleryDlEngine) CanHandle(_ string) error { return nil }

// Download runs gallery-dl and streams one ProgressUpdate per downloaded file.
func (e *galleryDlEngine) Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error {
	if req.DryRun {
		args := append([]string{"--simulate"}, e.buildArgs(req)...)
		cmd := exec.CommandContext(ctx, e.binaryPath, args...)
		out, _ := cmd.CombinedOutput()
		progress(ProgressUpdate{
			Filename:   strings.TrimSpace(string(out)),
			TotalBytes: -1,
			Done:       true,
		})
		return nil
	}

	args := e.buildArgs(req)
	cmd := exec.CommandContext(ctx, e.binaryPath, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("could not attach to gallery-dl stdout: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("could not attach to gallery-dl stderr: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not start gallery-dl: %w", err)
	}

	// Collect stderr for error classification on failure
	var stderrLines []string
	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if line := strings.TrimSpace(sc.Text()); line != "" {
				stderrLines = append(stderrLines, line)
			}
		}
	}()

	// Each stdout line is a completed file path — emit one ProgressUpdate per file
	var fileCount int64
	var lastFilename string

	sc := bufio.NewScanner(stdout)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		fileCount++
		lastFilename = filepath.Base(line)
		progress(ProgressUpdate{
			Filename:   lastFilename,
			TotalBytes: -1,        // total unknown upfront — progress bar uses file count
			DoneBytes:  fileCount, // repurposed as file counter in count mode
			Done:       false,
		})
	}

	<-stderrDone

	if err := cmd.Wait(); err != nil {
		return classifyGalleryDlError(stderrLines, err)
	}

	// gallery-dl exits 0 but wrote nothing to stdout — URL had no media
	// this is the image-only tweet case where yt-dlp was tried first
	if fileCount == 0 {
		return ErrNoMedia
	}

	progress(ProgressUpdate{
		Filename:  lastFilename,
		DoneBytes: fileCount,
		Done:      true,
	})

	return nil
}

// buildArgs constructs the gallery-dl argument list from a DownloadRequest.
func (e *galleryDlEngine) buildArgs(req DownloadRequest) []string {
	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = "."
	}

	args := []string{
		"-o", "output.mode=pipe",
		"-o", "output.shorten=false",
		"-o", "output.progress=false",
		// -D sets the exact output directory — no category/username subdirectories.
		// --dest / -d is the base directory and still creates twitter/grafana/ nesting.
		"-D", outputDir,
	}

	if req.NoMeta {
		args = append(args, "-o", "postprocessors=null")
	}

	args = append(args, req.URL)
	return args
}

// classifyGalleryDlError inspects stderr lines from a failed gallery-dl run
// and returns either ErrNoMedia (so the orchestrator can try a fallback engine)
// or a descriptive error for the user.
func classifyGalleryDlError(lines []string, execErr error) error {
	for _, line := range lines {
		lower := strings.ToLower(line)

		// Unsupported URL → ErrNoMedia so orchestrator can fall back
		if strings.Contains(lower, "unsupported url") {
			return ErrNoMedia
		}

		// Auth / login errors — actionable message, not a fallback case
		if strings.Contains(lower, "login") || strings.Contains(lower, "authentication") {
			return fmt.Errorf("gallery-dl failed: this gallery requires authentication — set credentials in ~/.config/gallery-dl/config.json")
		}

		// 404 / not found
		if strings.Contains(lower, "not found") || strings.Contains(lower, "404") {
			return fmt.Errorf("gallery-dl failed: gallery not found — the URL may be private, deleted, or incorrect")
		}

		// Rate limited
		if strings.Contains(lower, "rate limit") || strings.Contains(lower, "429") {
			return fmt.Errorf("gallery-dl failed: rate limited by the site — wait a few minutes and try again")
		}
	}

	// Generic fallback — surface the last non-empty stderr line
	for i := len(lines) - 1; i >= 0; i-- {
		if line := strings.TrimSpace(lines[i]); line != "" {
			return fmt.Errorf("gallery-dl failed: %s", line)
		}
	}

	return fmt.Errorf("gallery-dl failed: %w", execErr)
}
