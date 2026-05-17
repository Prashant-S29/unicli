// Copyright © 2026 Prashant Singh
package engines

import (
	"context"
	"time"
)

// Engine is the interface every download backend must implement.
// The orchestrator in internal/download/download.go calls these methods
// and never cares which concrete engine is running.
type Engine interface {
	// Name returns a short identifier used in logs and error messages.
	Name() string

	// CanHandle does a quick pre-flight check before committing to a download.
	// Returns nil if the engine is ready to proceed, or an error explaining why not.
	CanHandle(url string) error

	// Download executes the download, calling progress repeatedly with updates.
	// Implementations must respect ctx cancellation.
	Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error
}

// DownloadRequest carries all user-supplied parameters for a single download.
type DownloadRequest struct {
	URL       string
	OutputDir string // "" means current working directory
	Format    string // optional forced output format (e.g. "mp4", "mp3")
	Quality   string // optional quality hint (e.g. "1080p", "best")
	AudioOnly bool
	NoMeta    bool
	DryRun    bool
}

// ProgressFunc is called repeatedly during a download with the latest state.
// Implementations must never block — this is called from the download goroutine.
type ProgressFunc func(ProgressUpdate)

// ProgressUpdate carries a snapshot of download state at a point in time.
type ProgressUpdate struct {
	// Filename is the local filename being written.
	// May be empty until the server responds with Content-Disposition or the
	// URL path is resolved.
	Filename string

	// TotalBytes is the expected total size. -1 means unknown (no Content-Length).
	TotalBytes int64

	// DoneBytes is how many bytes have been written so far.
	DoneBytes int64

	// Speed is the current transfer rate in bytes per second.
	Speed float64

	// ETA is the estimated time remaining. Zero if unknown.
	ETA time.Duration

	// Done is true on the final update when the download has completed.
	Done bool
}
