package engines

import "context"

// ImageInfo holds all metadata read from an image file.
// Basic fields are always populated. Extended fields are populated
// only when a full probe is requested (--all).
type ImageInfo struct {
	// --- always present ---
	Filename      string
	Format        string // "WebP", "JPEG", "PNG" etc.
	Width         int
	Height        int
	FilesizeBytes int64

	// --- always present when ffprobe can read them ---
	ColorSpace string // e.g. "yuv420p", "rgb24"
	BitDepth   int    // bits per channel, 0 if unknown

	// --- extended (--all) ---
	// Raw is the complete ffprobe JSON output, used when --all is set.
	// The presenter formats it into labelled key/value lines.
	Raw map[string]interface{}
}

// ConvertRequest is the input to Engine.Convert for a single file.
type ConvertRequest struct {
	// InputPath is the absolute or relative path to the source file.
	InputPath string

	// OutputPath is the full destination path including filename and extension.
	// The orchestrator is responsible for computing this before calling Convert.
	OutputPath string

	// ToFormat is the target format, lowercase, e.g. "webp", "png", "jpeg".
	ToFormat string
}

// ConvertResult holds the outcome of a single file conversion.
type ConvertResult struct {
	InputPath  string
	OutputPath string
	// Err is nil on success.
	Err error
}

// ProgressFunc is called by Convert after each file finishes (success or fail).
// It receives the result so the caller can collect or display outcomes.
type ProgressFunc func(result ConvertResult)

// Engine is the interface all image processing engines must implement.
type Engine interface {
	// Name returns the engine identifier for logging.
	Name() string

	// Info reads image metadata from a single file.
	// If full is true, Raw is populated with everything ffprobe exposes.
	Info(file string, full bool) (*ImageInfo, error)

	// Convert transcodes a single image file per the request.
	// It calls fn once when the operation completes (success or failure).
	// ctx cancellation is respected — a cancelled context must return promptly.
	Convert(ctx context.Context, req ConvertRequest, fn ProgressFunc) error
}
