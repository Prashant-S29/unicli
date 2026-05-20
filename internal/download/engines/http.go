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
type httpEngine struct {
	client *http.Client
}

func NewHTTPEngine() Engine {
	return &httpEngine{
		client: &http.Client{
			Timeout: 0,
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
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}

func (e *httpEngine) Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error {
	if req.DryRun {
		filename := filenameFromURL(req.URL, "")
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

	// HEAD request — get size, Content-Type, Accept-Ranges, Content-Disposition
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, req.URL, nil)
	if err != nil {
		return fmt.Errorf("bad URL: %w", err)
	}
	headReq.Header.Set("User-Agent", "unicli")

	var headResp *http.Response
	headResp, err = e.client.Do(headReq)
	if err == nil {
		headResp.Body.Close()
	}

	// Determine filename — Content-Disposition first, then URL path,
	// then fix up missing extension from Content-Type
	filename := ""
	contentType := ""
	if headResp != nil {
		contentType = headResp.Header.Get("Content-Type")
		filename = filenameFromResponse(headResp, req.URL)
	}
	if filename == "" {
		filename = filenameFromURL(req.URL, contentType)
	} else if filepath.Ext(filename) == "" {
		// Content-Disposition gave us a name but no extension — fix it up
		filename = addExtFromContentType(filename, contentType)
	}

	destPath := filepath.Join(outputDir, filename)

	// Resume support
	var startByte int64
	if info, err := os.Stat(destPath); err == nil {
		startByte = info.Size()
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, req.URL, nil)
	if err != nil {
		return fmt.Errorf("bad URL: %w", err)
	}
	getReq.Header.Set("User-Agent", "unicli")

	supportsRange := headResp != nil && strings.EqualFold(headResp.Header.Get("Accept-Ranges"), "bytes")
	if startByte > 0 && supportsRange {
		getReq.Header.Set("Range", fmt.Sprintf("bytes=%d-", startByte))
	} else {
		startByte = 0
	}

	resp, err := e.client.Do(getReq)
	if err != nil {
		return fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusOK && startByte > 0 {
		startByte = 0
	}

	// If HEAD failed or returned no Content-Type, get it from the GET response
	if contentType == "" {
		contentType = resp.Header.Get("Content-Type")
	}

	// Re-derive filename from GET response if HEAD wasn't available
	if headResp == nil {
		filename = filenameFromResponse(resp, req.URL)
		if filename == "" {
			filename = filenameFromURL(req.URL, contentType)
		} else if filepath.Ext(filename) == "" {
			filename = addExtFromContentType(filename, contentType)
		}
		destPath = filepath.Join(outputDir, filename)
	}

	totalBytes := int64(-1)
	if resp.ContentLength > 0 {
		totalBytes = resp.ContentLength + startByte
	}

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

	buf := make([]byte, 32*1024)
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

// contentTypeToExt maps MIME types to file extensions.
// Only types realistically served as direct downloads are listed.
var contentTypeToExt = map[string]string{
	"image/jpeg":           ".jpg",
	"image/jpg":            ".jpg",
	"image/png":            ".png",
	"image/gif":            ".gif",
	"image/webp":           ".webp",
	"image/svg+xml":        ".svg",
	"image/bmp":            ".bmp",
	"image/tiff":           ".tiff",
	"image/avif":           ".avif",
	"video/mp4":            ".mp4",
	"video/webm":           ".webm",
	"video/quicktime":      ".mov",
	"video/x-matroska":     ".mkv",
	"video/avi":            ".avi",
	"audio/mpeg":           ".mp3",
	"audio/mp4":            ".m4a",
	"audio/ogg":            ".ogg",
	"audio/flac":           ".flac",
	"audio/wav":            ".wav",
	"application/pdf":      ".pdf",
	"application/zip":      ".zip",
	"application/epub+zip": ".epub",
}

// extFromContentType returns the file extension for a Content-Type header value.
// Strips parameters (e.g. "image/jpeg; charset=utf-8" → "image/jpeg") before lookup.
// Returns "" if the type is unknown or is text/html (not a downloadable file).
func extFromContentType(ct string) string {
	if ct == "" {
		return ""
	}
	// Strip parameters
	if i := strings.Index(ct, ";"); i >= 0 {
		ct = ct[:i]
	}
	ct = strings.TrimSpace(strings.ToLower(ct))

	// html is never a real download target — don't give it an extension
	if ct == "text/html" || ct == "text/plain" {
		return ""
	}
	return contentTypeToExt[ct]
}

// addExtFromContentType appends the correct extension to name if one can be
// derived from the Content-Type header and name doesn't already have one.
func addExtFromContentType(name, contentType string) string {
	if filepath.Ext(name) != "" {
		return name // already has an extension
	}
	ext := extFromContentType(contentType)
	if ext == "" {
		return name // unknown type — leave as-is, don't append .unknown_video
	}
	return name + ext
}

// filenameFromResponse tries Content-Disposition first, then falls back.
// The returned name has its extension fixed up from Content-Type if missing.
func filenameFromResponse(resp *http.Response, rawURL string) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd != "" {
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

// filenameFromURL extracts the last path segment and fixes up the extension
// from contentType if the segment has none.
func filenameFromURL(rawURL string, contentType string) string {
	// Strip query and fragment
	if i := strings.Index(rawURL, "?"); i >= 0 {
		rawURL = rawURL[:i]
	}
	if i := strings.Index(rawURL, "#"); i >= 0 {
		rawURL = rawURL[:i]
	}
	// Last path segment
	if i := strings.LastIndex(rawURL, "/"); i >= 0 {
		rawURL = rawURL[i+1:]
	}
	if rawURL == "" {
		rawURL = "download"
	}
	name := sanitizeFilename(rawURL)
	return addExtFromContentType(name, contentType)
}

// sanitizeFilename strips characters illegal in filenames on common OSes.
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
