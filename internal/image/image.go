package image

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/prashant-s29/unicli/internal/config"
	imgengines "github.com/prashant-s29/unicli/internal/image/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// InfoRequest is the input to RunInfo.
type InfoRequest struct {
	Targets []string
	Full    bool
	Quiet   bool
	Verbose bool
}

// RunInfo is the orchestrator for `unicli image info`.
func RunInfo(req InfoRequest) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	// Resolve ffmpeg engine
	engine, managed, err := imgengines.NewFFmpegEngine(cfg.Engines.BinDir)
	if err != nil {
		return fmt.Errorf("could not resolve ffmpeg: %w", err)
	}
	if engine == nil {
		return promptInstallFFmpeg(managed, cfg.Engines.BinDir)
	}

	// Detect files
	result := Detect(req.Targets, DetectOptions{})

	if len(result.Files) == 0 && len(result.Skipped) == 0 {
		ui.Info("No images found")
		return nil
	}

	// Process each file
	var failed []SkippedFile

	for i, f := range result.Files {
		if i > 0 {
			ui.Blank()
		}

		info, err := engine.Info(f.Path, req.Full)
		if err != nil {
			failed = append(failed, SkippedFile{Path: f.Path, Reason: err.Error()})
			if !req.Quiet {
				printInfoError(f.Path, err)
			}
			continue
		}

		if !req.Quiet {
			if req.Full {
				printInfoFull(info)
			} else {
				printInfoBasic(info)
			}
		}
	}

	// Report skipped + failed at the end if more than one file
	if len(result.Files)+len(result.Skipped) > 1 {
		printInfoSummary(result.Skipped, failed)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d file(s) could not be read", len(failed))
	}

	return nil
}

// ---- Output renderers ----------------------------------------------------

func printInfoBasic(info *imgengines.ImageInfo) {
	label := ui.StyleLabel.Render
	fmt.Printf("  %s %s\n", label("File"), filepath.Base(info.Filename))
	fmt.Printf("  %s %s\n", label("Format"), info.Format)
	fmt.Printf("  %s %d × %d\n", label("Size"), info.Width, info.Height)
	fmt.Printf("  %s %s\n", label("Filesize"), imgengines.HumanSize(info.FilesizeBytes))
	if info.ColorSpace != "" {
		fmt.Printf("  %s %s\n", label("Color"), info.ColorSpace)
	}
}

func printInfoFull(info *imgengines.ImageInfo) {
	// Start with the basic block
	printInfoBasic(info)

	if info.BitDepth > 0 {
		fmt.Printf("  %s %d-bit\n", ui.StyleLabel.Render("Bit depth"), info.BitDepth)
	}

	if info.Raw == nil {
		return
	}

	// EXIF / metadata tags — injected as _tags by ffmpeg.go
	if rawTags, ok := info.Raw["_tags"]; ok {
		if tags, ok := rawTags.(map[string]string); ok && len(tags) > 0 {
			ui.Blank()
			fmt.Printf("  %s\n", ui.StyleBold.Render("Metadata"))

			// Sort keys for stable output
			keys := make([]string, 0, len(tags))
			for k := range tags {
				keys = append(keys, k)
			}
			sort.Strings(keys)

			for _, k := range keys {
				label := ui.StyleLabel.Copy().Width(22).Render(formatTagKey(k))
				fmt.Printf("  %s %s\n", label, tags[k])
			}
		}
	}

	// Stream-level details from the raw ffprobe JSON
	if streams, ok := info.Raw["streams"]; ok {
		if streamList, ok := streams.([]interface{}); ok && len(streamList) > 0 {
			ui.Blank()
			fmt.Printf("  %s\n", ui.StyleBold.Render("Stream"))
			if s, ok := streamList[0].(map[string]interface{}); ok {
				printRawSection(s, []string{
					"codec_name", "codec_long_name", "profile",
					"pix_fmt", "color_space", "color_range",
					"bits_per_raw_sample", "r_frame_rate",
				})
			}
		}
	}

	// Format-level details
	if format, ok := info.Raw["format"]; ok {
		if f, ok := format.(map[string]interface{}); ok {
			ui.Blank()
			fmt.Printf("  %s\n", ui.StyleBold.Render("Format"))
			printRawSection(f, []string{
				"format_name", "format_long_name",
				"duration", "bit_rate", "nb_streams",
			})
		}
	}
}

// printRawSection renders a subset of keys from a raw map as label/value lines.
func printRawSection(m map[string]interface{}, keys []string) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		s := fmt.Sprintf("%v", v)
		if s == "" || s == "0" || s == "unknown" {
			continue
		}
		label := ui.StyleLabel.Copy().Width(22).Render(formatTagKey(k))
		fmt.Printf("  %s %s\n", label, s)
	}
}

// formatTagKey converts snake_case or EXIF-style keys to Title Case for display.
func formatTagKey(k string) string {
	k = strings.ReplaceAll(k, "_", " ")
	words := strings.Fields(k)
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func printInfoError(path string, err error) {
	fmt.Printf("  %s  %s\n",
		ui.StyleError.Render(ui.SymbolError),
		ui.StyleBold.Render(filepath.Base(path)),
	)
	fmt.Printf("     %s %s\n", ui.StyleLabel.Render("Reason:"), err.Error())
}

func printInfoSummary(skipped []SkippedFile, failed []SkippedFile) {
	if len(skipped) == 0 && len(failed) == 0 {
		return
	}
	ui.Blank()
	for _, s := range skipped {
		fmt.Printf("  %s  %-30s %s\n",
			ui.StyleWarning.Render("⊘"),
			filepath.Base(s.Path),
			ui.StyleMuted.Render("skipped  ("+s.Reason+")"),
		)
	}
	for _, f := range failed {
		fmt.Printf("  %s  %-30s %s\n",
			ui.StyleError.Render(ui.SymbolError),
			filepath.Base(f.Path),
			ui.StyleMuted.Render("failed   ("+f.Reason+")"),
		)
	}
}

// promptInstallFFmpeg surfaces the inline install prompt when ffmpeg is missing.
func promptInstallFFmpeg(_ bool, _ string) error {
	ui.Blank()
	ui.Error(
		"ffmpeg is required for image operations",
		"ffmpeg was not found on this system",
		"run `unicli setup` to install it automatically",
	)
	ui.Blank()

	// Check if macOS — give brew hint
	if isMacOS() {
		ui.Muted("Or install manually:  brew install ffmpeg")
	}
	ui.Blank()
	return fmt.Errorf("ffmpeg not found")
}

func isMacOS() bool {
	_, err := os.Stat("/usr/local/bin/brew")
	if err == nil {
		return true
	}
	_, err = os.Stat("/opt/homebrew/bin/brew")
	return err == nil
}
