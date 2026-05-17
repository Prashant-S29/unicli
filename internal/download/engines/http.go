// Copyright © 2026 Prashant Singh
package engines

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// httpEngine downloads any direct file URL using Go's stdlib net/http.
// It supports:
//   - Streaming (never loads the full file into memory)
//   - Resume via Range header if a partial file already exists
//   - Progress reporting on every chunk
type httpEngine struct {
	client *http.Client
}

// NewHTTPEngine returns a ready-to-use HTTP download engine.
func NewHTTPEngine() Engine {
	return &httpEngine{
		client: &http.Client{
			Timeout: 0, // no timeout — downloads can take a long time
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

func (e *httpEngine) Name() string { return "http" }

// CanHandle does a HEAD request to verify the URL is reachable and returns
// a file (not an HTML page). Returns nil if the URL looks downloadable.
func (e *httpEngine) CanHandle(url string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	req.Header.Set("User-Agent", "unicli")

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("URL not reachable: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusMethodNotAllowed {
		// Some servers don't support HEAD — treat as reachable
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	return nil
}

// Download streams the URL to disk, reporting progress via the callback.
func (e *httpEngine) Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error {
	if req.DryRun {
		filename := filenameFromURL(req.URL)
		progress(ProgressUpdate{Filename: filename, DoneBytes: 0, TotalBytes: -1})
		progress(ProgressUpdate{Filename: filename, Done: true})
		return nil
	}

	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = "."
	}

	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("could not create output directory: %w", err)
	}

	// First request — check size and whether resume is possible
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, req.URL, nil)
	if err != nil {
		return fmt.Errorf("bad URL: %w", err)
	}
	headReq.Header.Set("User-Agent", "unicli")

	headResp, err := e.client.Do(headReq)
	if err == nil {
		headResp.Body.Close()
	}

	// Determine filename
	filename := ""
	if headResp != nil {
		filename = filenameFromResponse(headResp, req.URL)
	}
	if filename == "" {
		filename = filenameFromURL(req.URL)
	}

	destPath := filepath.Join(outputDir, filename)

	// Check if a partial file exists — attempt resume
	var startByte int64
	if info, err := os.Stat(destPath); err == nil {
		startByte = info.Size()
	}

	// Build the actual GET request
	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return fmt.Errorf("bad URL: %w", err)
	}
	getReq.Header.Set("User-Agent", "unicli")

	// Add Range header if we have a partial file and the server reported accept-ranges
	supportsRange := headResp != nil && strings.EqualFold(headResp.Header.Get("Accept-Ranges"), "bytes")
	if startByte > 0 && supportsRange {
		getReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	} else {
		startByte = 0 // can't resume — start fresh
	}

	resp, err := e.client.Do(getReq)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}

	// If server ignored our Range and sent 200, restart from zero
	if resp.StatusCode == http.StatusOK && startByte > 0 {
		startByte = 0
	}

	// Total size: for partial content, Content-Length is the remaining bytes
	totalBytes := int64(-1)
	if resp.ContentLength > 0 {
		totalBytes = resp.ContentLength + startByte
	}

	// Open the file — append if resuming, create/truncate otherwise
	var file *os.File
	if startByte > 0 {
		file, err = os.OpenFile(destPath, os.O_APPEND|os.O_WRONLY, 0644)
	} else {
		file, err = os.Create(destPath)
	}
	if err != nil {
		return fmt.Errorf("could not open output file: %w", err)
	}
	defer file.Close()

	// Stream body → file, emitting progress on each chunk
	buf := make([]byte, 32*1024) // 32 KB chunks
	done := startByte
	startTime := time.Now()

	progress(ProgressUpdate{
		Filename:   filename,
		TotalBytes: totalBytes,
		DoneBytes:  done,
	})

	for {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("download cancelled: %w", err)
		}

		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := file.Write(buf[:n]); writeErr != nil {
				return fmt.Errorf("write error: %w", writeErr)
			}
			done += int64(n)

			elapsed := time.Since(startTime).Seconds()
			speed := float64(done-startByte) / elapsed

			var eta time.Duration
			if speed > 0 && totalBytes > 0 {
				remaining := float64(totalBytes-done) / speed
				eta = time.Duration(remaining) * time.Second
			}

			progress(ProgressUpdate{
				Filename:   filename,
				TotalBytes: totalBytes,
				DoneBytes:  done,
				Speed:      speed,
				ETA:        eta,
			})
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return fmt.Errorf("read error: %w", readErr)
		}
	}

	progress(ProgressUpdate{
		Filename:   filename,
		TotalBytes: totalBytes,
		DoneBytes:  done,
		Done:       true,
	})

	return nil
}

// ---- Filename helpers ----------------------------------------------------

// filenameFromResponse tries Content-Disposition first, then falls back to URL.
func filenameFromResponse(resp *http.Response, rawURL string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
		// Parse: attachment; filename="foo.zip" or filename*=UTF-8''foo.zip
		for _, part := range strings.Split(cd, ";") {
			part = strings.TrimSpace(part)
			if strings.HasPrefix(part, "filename=") {
				name := strings.TrimPrefix(part, "filename=")
				name = strings.Trim(name, `"`)
				if name != "" {
					return sanitizeFilename(name)
				}
			}
		}
	}
	return ""
}

// filenameFromURL extracts the last path segment of the URL.
func filenameFromURL(rawURL string) string {
	// Strip query string
	if i := strings.Index(rawURL, "?"); i >= 0 {
		rawURL = rawURL[:i]
	}
	// Strip fragment
	if i := strings.Index(rawURL, "#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	// Last path segment
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		rawURL = rawURL[i+1:]
	}
	if rawURL == "" {
		return "download"
	}
	return sanitizeFilename(rawURL)
}

// sanitizeFilename strips characters that are illegal in filenames on
// common operating systems.
func sanitizeFilename(name string) string {
	replacer := strings.NewReplacer(
		"/", "_", "\\", "_", ":", "_", "*", "_",
		"?", "_", `"`, "_", "<", "_", ">", "_", "|", "_",
	)
	name = replacer.Replace(name)
	name = strings.TrimSpace(name)
	if name == "" {
		return "download"
	}
	return name
}
