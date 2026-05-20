package engines

import (
	"context"
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
	ffmpegPath  string
}

// NewFFmpegEngine resolves the ffprobe binary and returns a ready engine.
// Returns (nil, false, nil) if the binary is not installed.
func NewFFmpegEngine(binDir string) (Engine, bool, error) {
	path, managed, err := mgr.Resolve(mgr.EngineFFmpeg, binDir)
	if err != nil {
		return nil, false, err
	}
	if path == "" {
		return nil, false, nil
	}

	ffprobePath := deriveFfprobePath(path)

	return &ffmpegEngine{
		ffprobePath: ffprobePath,
		ffmpegPath:  path,
	}, managed, nil
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
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	return parseFFprobeOutput(file, out, full)
}

// ---- Convert -------------------------------------------------------------

// codecFlag maps a lowercase target format to the ffmpeg codec/encoder flag
// value passed via -vcodec. Formats not in this map are unsupported.
var codecFlag = map[string]string{
	"jpeg": "mjpeg",
	"jpg":  "mjpeg",
	"png":  "png",
	"webp": "libwebp",
	"bmp":  "bmp",
	"tiff": "tiff",
	"gif":  "gif",
	"avif": "libaom-av1",
}

// Convert transcodes a single image file using ffmpeg.
// Calls fn exactly once when the operation finishes (success or failure).
func (e *ffmpegEngine) Convert(ctx context.Context, req ConvertRequest, fn ProgressFunc) error {
	codec, ok := codecFlag[strings.ToLower(req.ToFormat)]
	if !ok {
		err := fmt.Errorf("unsupported output format: %s", req.ToFormat)
		fn(ConvertResult{InputPath: req.InputPath, OutputPath: req.OutputPath, Err: err})
		return err
	}

	args := []string{
		"-y", // overwrite output without prompting
		"-i", req.InputPath,
		"-vcodec", codec,
		"-vframes", "1", // images are single-frame; avoids multi-frame confusion
		req.OutputPath,
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)

	// ffmpeg writes all its chatter to stderr. Capture it so we can surface
	// it on failure without polluting stdout during normal operation.
	out, err := cmd.CombinedOutput()
	if err != nil {
		wrapped := fmt.Errorf("ffmpeg failed: %w\n%s", err, trimFFmpegOutput(out))
		fn(ConvertResult{InputPath: req.InputPath, OutputPath: req.OutputPath, Err: wrapped})
		return wrapped
	}

	fn(ConvertResult{InputPath: req.InputPath, OutputPath: req.OutputPath, Err: nil})
	return nil
}

// trimFFmpegOutput keeps only the last 5 lines of ffmpeg stderr output.
// ffmpeg is very chatty; the error is always at the bottom.
func trimFFmpegOutput(b []byte) string {
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) > 5 {
		lines = lines[len(lines)-5:]
	}
	return strings.Join(lines, "\n")
}

// ---- ffprobe JSON parsing ------------------------------------------------

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

	var filesize int64
	if probe.Format.Size != "" {
		filesize, _ = strconv.ParseInt(probe.Format.Size, 10, 64)
	}

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
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err == nil {
			info.Raw = raw
		}

		tags := make(map[string]string)
		for k, v := range probe.Format.Tags {
			tags[k] = v
		}
		for k, v := range stream.Tags {
			tags[k] = v
		}
		if len(tags) > 0 {
			info.Raw["_tags"] = tags
		}
	}

	return info, nil
}

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
	if formatName != "" {
		return strings.ToUpper(strings.Split(formatName, ",")[0])
	}
	return strings.ToUpper(codecName)
}

func deriveFfprobePath(ffmpegPath string) string {
	dir := ffmpegPath[:strings.LastIndex(ffmpegPath, "/")+1]
	if strings.Contains(ffmpegPath, "\\") {
		dir = ffmpegPath[:strings.LastIndex(ffmpegPath, "\\")+1]
		return dir + "ffprobe.exe"
	}
	if dir == "" {
		return "ffprobe"
	}
	return dir + "ffprobe"
}

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
	rounded := math.Round(size*10) / 10
	return fmt.Sprintf("%.1f %s", rounded, units[i])
}
