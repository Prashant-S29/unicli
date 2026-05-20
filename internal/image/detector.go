package image

import (
	"os"
	"path/filepath"
	"strings"
)

// SupportedFormats is the single source of truth for all formats unicli
// supports. Used for both runtime validation and autocomplete completions.
var SupportedFormats = []string{
	"jpeg", "jpg", "png", "webp", "bmp", "tiff", "gif", "avif",
}

// supportedExts is the same list keyed by extension (with dot) for O(1) lookup.
var supportedExts = func() map[string]struct{} {
	m := make(map[string]struct{}, len(SupportedFormats))
	for _, f := range SupportedFormats {
		m["."+f] = struct{}{}
	}
	return m
}()

// FileResult holds a single file that came out of detection.
type FileResult struct {
	Path string
}

// SkippedFile holds a file that was excluded before processing, with a reason.
type SkippedFile struct {
	Path   string
	Reason string
}

// DetectResult is the full output of a Detect call.
type DetectResult struct {
	Files   []FileResult
	Skipped []SkippedFile
}

// DetectOptions controls how Detect enumerates files.
type DetectOptions struct {
	// FromFormats filters to only these extensions. nil = all supported formats.
	FromFormats []string
	// Recursive descends into subdirectories.
	Recursive bool
}

// Detect resolves targets into a DetectResult.
//
// targets can be:
//   - empty              → current directory
//   - a single file path
//   - multiple file paths or globs
//   - a directory path
func Detect(targets []string, opts DetectOptions) DetectResult {
	if len(targets) == 0 {
		targets = []string{"."}
	}

	// Build the allowed-extension set for this call.
	allowedExts := supportedExts
	if len(opts.FromFormats) > 0 {
		allowedExts = make(map[string]struct{}, len(opts.FromFormats))
		for _, f := range opts.FromFormats {
			allowedExts["."+strings.ToLower(f)] = struct{}{}
		}
	}

	var result DetectResult
	seen := make(map[string]struct{})

	for _, target := range targets {
		info, err := os.Stat(target)
		if err != nil {
			result.Skipped = append(result.Skipped, SkippedFile{
				Path:   target,
				Reason: "file not found",
			})
			continue
		}

		if info.IsDir() {
			walkDir(target, opts.Recursive, allowedExts, seen, &result)
		} else {
			addFile(target, allowedExts, seen, &result)
		}
	}

	return result
}

// walkDir enumerates image files inside a directory.
func walkDir(dir string, recursive bool, allowed map[string]struct{}, seen map[string]struct{}, result *DetectResult) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		result.Skipped = append(result.Skipped, SkippedFile{
			Path:   dir,
			Reason: "could not read directory",
		})
		return
	}

	for _, e := range entries {
		path := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if recursive {
				walkDir(path, recursive, allowed, seen, result)
			}
			continue
		}
		addFile(path, allowed, seen, result)
	}
}

// addFile checks a single file and appends it to Files or Skipped.
func addFile(path string, allowed map[string]struct{}, seen map[string]struct{}, result *DetectResult) {
	// Deduplicate
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if _, ok := seen[abs]; ok {
		return
	}
	seen[abs] = struct{}{}

	ext := strings.ToLower(filepath.Ext(path))

	// Not an image extension we know at all
	if _, inSupported := supportedExts[ext]; !inSupported {
		// Only report skip if it looked like it might be an image
		// (has any extension). Silent skip for .DS_Store, .txt etc.
		if ext != "" && looksLikeImage(ext) {
			result.Skipped = append(result.Skipped, SkippedFile{
				Path:   path,
				Reason: strings.TrimPrefix(ext, ".") + " not supported",
			})
		}
		return
	}

	// Extension is supported globally but filtered out by --from
	if _, inAllowed := allowed[ext]; !inAllowed {
		return
	}

	result.Files = append(result.Files, FileResult{Path: path})
}

// looksLikeImage returns true for extensions that are plausibly image formats
// even if we don't support them — used to decide whether to report a skip.
var knownImageExts = map[string]struct{}{
	".heic": {}, ".heif": {}, ".raw": {}, ".cr2": {}, ".nef": {},
	".arw": {}, ".dng": {}, ".psd": {}, ".xcf": {}, ".svg": {},
	".ico": {}, ".jfif": {}, ".jp2": {}, ".j2k": {},
}

func looksLikeImage(ext string) bool {
	_, ok := knownImageExts[ext]
	return ok
}
