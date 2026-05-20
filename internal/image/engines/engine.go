package engines

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

// Engine is the interface all image processing engines must implement.
// For M9a only Info is required; Convert and future ops are added in later milestones.
type Engine interface {
	// Name returns the engine identifier for logging.
	Name() string

	// Info reads image metadata from a single file.
	// If full is true, Raw is populated with everything ffprobe exposes.
	Info(file string, full bool) (*ImageInfo, error)
}
