package engines

import (
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"strconv"
	"strings"

	mgr "github.com/prashant-s29/unicli/internal/engines"
)

// ffmpegEngine implements Engine using ffprobe for reads and ffmpeg for writes.
type ffmpegEngine struct {
	ffprobePath string
}

// NewFFmpegEngine resolves the ffprobe binary and returns a ready engine.
// Returns (nil, false, nil) if the binary is not installed.
func NewFFmpegEngine(binDir string) (Engine, bool, error) {
	// ffprobe ships alongside ffmpeg — resolve it the same way.
	path, managed, err := mgr.Resolve(mgr.EngineFFmpeg, binDir)
	if err != nil {
		return nil, false, err
	}
	if path == "" {
		return nil, false, nil
	}

	// The managed binary is named "ffmpeg". ffprobe sits next to it.
	ffprobePath := deriveFfprobePath(path)

	return &ffmpegEngine{ffprobePath: ffprobePath}, managed, nil
}

func (e *ffmpegEngine) Name() string { return mgr.EngineFFmpeg }

// Info runs ffprobe on a single file and returns structured metadata.
func (e *ffmpegEngine) Info(file string, full bool) (*ImageInfo, error) {
	args := []string{
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		file,
	}

	cmd := exec.Command(e.ffprobePath, args...)
	out, err := cmd.Output()
	if err != nil {
		// ffprobe exits non-zero for unreadable files
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	return parseFFprobeOutput(file, out, full)
}

// ---- ffprobe JSON parsing ------------------------------------------------

// ffprobeOutput mirrors the JSON structure ffprobe emits.
type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType        string            `json:"codec_type"`
	CodecName        string            `json:"codec_name"`
	Width            int               `json:"width"`
	Height           int               `json:"height"`
	PixFmt           string            `json:"pix_fmt"`
	BitsPerRawSample string            `json:"bits_per_raw_sample"`
	Tags             map[string]string `json:"tags"`
}

type ffprobeFormat struct {
	Filename       string            `json:"filename"`
	FormatName     string            `json:"format_name"`
	FormatLongName string            `json:"format_long_name"`
	Size           string            `json:"size"`
	Tags           map[string]string `json:"tags"`
}

func parseFFprobeOutput(file string, data []byte, full bool) (*ImageInfo, error) {
	var probe ffprobeOutput
	if err := json.Unmarshal(data, &probe); err != nil {
		return nil, fmt.Errorf("could not parse ffprobe output: %w", err)
	}

	// Find the video/image stream
	var stream *ffprobeStream
	for i := range probe.Streams {
		if probe.Streams[i].CodecType == "video" {
			stream = &probe.Streams[i]
			break
		}
	}
	if stream == nil {
		return nil, fmt.Errorf("no image stream found in %s", file)
	}

	// Filesize
	var filesize int64
	if probe.Format.Size != "" {
		filesize, _ = strconv.ParseInt(probe.Format.Size, 10, 64)
	}

	// Bit depth
	bitDepth := 0
	if stream.BitsPerRawSample != "" && stream.BitsPerRawSample != "0" {
		bitDepth, _ = strconv.Atoi(stream.BitsPerRawSample)
	}

	info := &ImageInfo{
		Filename:      file,
		Format:        formatName(stream.CodecName, probe.Format.FormatName),
		Width:         stream.Width,
		Height:        stream.Height,
		FilesizeBytes: filesize,
		ColorSpace:    stream.PixFmt,
		BitDepth:      bitDepth,
	}

	if full {
		// Build the raw map from the full ffprobe output for --all display.
		// We unmarshal into a generic map so the presenter can iterate all keys.
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			info.Raw = raw
		}

		// Also fold EXIF tags from both stream and format into a flat tags map
		// so the presenter doesn't need to know the ffprobe structure.
		tags := make(map[string]string)
		for k, v := range probe.Format.Tags {
			tags[k] = v
		}
		for k, v := range stream.Tags {
			tags[k] = v
		}
		if len(tags) > 0 {
			// Inject flattened tags back into raw for the presenter
			info.Raw["_tags"] = tags
		}
	}

	return info, nil
}

// formatName converts ffprobe's internal codec/format names to a display name.
func formatName(codecName, formatName string) string {
	switch codecName {
	case "mjpeg", "jpeg":
		return "JPEG"
	case "png":
		return "PNG"
	case "webp":
		return "WebP"
	case "gif":
		return "GIF"
	case "bmp":
		return "BMP"
	case "tiff":
		return "TIFF"
	case "av1", "avif":
		return "AVIF"
	}
	// Fall back to format_name, uppercased
	if formatName != "" {
		return strings.ToUpper(strings.Split(formatName, ",")[0])
	}
	return strings.ToUpper(codecName)
}

// deriveFfprobePath infers the ffprobe binary path from the ffmpeg binary path.
// They always ship together in the same directory.
func deriveFfprobePath(ffmpegPath string) string {
	// Replace the binary name, preserve directory and any .exe suffix
	dir := ffmpegPath[:strings.LastIndex(ffmpegPath, "/")+1]
	if strings.Contains(ffmpegPath, "\\") {
		// Windows path
		dir = ffmpegPath[:strings.LastIndex(ffmpegPath, "\\")+1]
		return dir + "ffprobe.exe"
	}
	if dir == "" {
		return "ffprobe"
	}
	return dir + "ffprobe"
}

// HumanSize formats bytes into a human-readable string.
func HumanSize(bytes int64) string {
	if bytes <= 0 {
		return "unknown"
	}
	units := []string{"B", "KB", "MB", "GB"}
	size := float64(bytes)
	i := 0
	for size >= 1024 && i < len(units)-1 {
		size /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%d B", bytes)
	}
	// Round to 1 decimal
	rounded := math.Round(size*10) / 10
	return fmt.Sprintf("%.1f %s", rounded, units[i])
}
