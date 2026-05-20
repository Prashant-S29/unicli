package image

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/prashant-s29/unicli/internal/config"
	imgengines "github.com/prashant-s29/unicli/internal/image/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// ---- Info ----------------------------------------------------------------

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

	engine, managed, err := imgengines.NewFFmpegEngine(cfg.Engines.BinDir)
	if err != nil {
		return fmt.Errorf("could not resolve ffmpeg: %w", err)
	}
	if engine == nil {
		return promptInstallFFmpeg(managed, cfg.Engines.BinDir)
	}

	result := Detect(req.Targets, DetectOptions{})

	if len(result.Files) == 0 && len(result.Skipped) == 0 {
		ui.Info("No images found")
		return nil
	}

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

	if len(result.Files)+len(result.Skipped) > 1 {
		printInfoSummary(result.Skipped, failed)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d file(s) could not be read", len(failed))
	}

	return nil
}

// ---- Convert -------------------------------------------------------------

// ConvertRequest is the input to RunConvert.
type ConvertRequest struct {
	Targets     []string
	ToFormat    string
	FromFormats []string // nil = all supported formats
	OutputDir   string   // "" = alongside originals
	Replace     bool     // overwrite originals in place
	Recursive   bool
	DryRun      bool
	Yes         bool // skip confirmation prompt
	Quiet       bool
	Verbose     bool
}

type job struct {
	input  string
	output string
}

// RunConvert is the orchestrator for `unicli image convert`.
func RunConvert(req ConvertRequest) error {
	// --- preflight ---------------------------------------------------------

	if req.Replace && req.OutputDir != "" {
		return fmt.Errorf("--replace and --output cannot be used together")
	}

	toFmt := strings.ToLower(strings.TrimPrefix(req.ToFormat, "."))
	if !isSupportedFormat(toFmt) {
		return fmt.Errorf("unsupported output format %q — supported: %s",
			req.ToFormat, strings.Join(SupportedFormats, ", "))
	}

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	engine, managed, err := imgengines.NewFFmpegEngine(cfg.Engines.BinDir)
	if err != nil {
		return fmt.Errorf("could not resolve ffmpeg: %w", err)
	}
	if engine == nil {
		return promptInstallFFmpeg(managed, cfg.Engines.BinDir)
	}

	// --- detect files ------------------------------------------------------

	detected := Detect(req.Targets, DetectOptions{
		FromFormats: req.FromFormats,
		Recursive:   req.Recursive,
	})

	batch := &BatchResult{}

	// Files that were skipped by the detector go straight into the batch
	// so they appear in the final summary.
	for _, s := range detected.Skipped {
		batch.skipped = append(batch.skipped, s)
	}

	// Filter out files whose extension already matches the target format —
	// no point converting png → png.
	var toProcess []FileResult
	for _, f := range detected.Files {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(f.Path), "."))
		// treat jpg and jpeg as the same
		if normaliseFormat(ext) == normaliseFormat(toFmt) {
			batch.skipped = append(batch.skipped, SkippedFile{
				Path:   f.Path,
				Reason: "already " + toFmt,
			})
			continue
		}
		toProcess = append(toProcess, f)
	}

	if len(toProcess) == 0 {
		batch.Print()
		return nil
	}

	// --- resolve output paths ----------------------------------------------

	var jobs []job

	for _, f := range toProcess {
		out, err := resolveOutputPath(f.Path, toFmt, req.OutputDir, req.Replace)
		if err != nil {
			batch.skipped = append(batch.skipped, SkippedFile{
				Path:   f.Path,
				Reason: err.Error(),
			})
			continue
		}
		jobs = append(jobs, job{input: f.Path, output: out})
	}

	// --- dry-run -----------------------------------------------------------

	if req.DryRun {
		printDryRun(jobs, batch.skipped)
		return nil
	}

	// --- confirmation prompt (batch only) ----------------------------------

	if len(jobs) > 1 && !req.Yes {
		if !confirmBatch(len(jobs), toFmt) {
			ui.Info("Cancelled.")
			return nil
		}
	}

	// --- ensure output directory exists ------------------------------------

	if req.OutputDir != "" {
		if err := os.MkdirAll(req.OutputDir, 0o755); err != nil {
			return fmt.Errorf("could not create output directory: %w", err)
		}
	}

	// --- run conversions ---------------------------------------------------

	ctx := context.Background()

	for _, j := range jobs {
		_ = engine.Convert(ctx, imgengines.ConvertRequest{
			InputPath:  j.input,
			OutputPath: j.output,
			ToFormat:   toFmt,
		}, batch.RecordConvert)
	}

	// --- delete originals if --replace ------------------------------------

	if req.Replace {
		for _, j := range jobs {
			// Only delete if conversion succeeded — check batch results
			if batch.wasConverted(j.input) {
				os.Remove(j.input)
			}
		}
	}

	// --- print summary -----------------------------------------------------

	if !req.Quiet {
		ui.Blank()
		batch.Print()
	}

	if batch.HasErrors() {
		return fmt.Errorf("%d file(s) failed to convert", len(batch.failed))
	}

	return nil
}

// ---- helpers -------------------------------------------------------------

func resolveOutputPath(inputPath, toFmt, outputDir string, replace bool) (string, error) {
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	filename := base + "." + toFmt

	if replace {
		// Output sits in the same directory as the input, replacing the original.
		return filepath.Join(filepath.Dir(inputPath), filename), nil
	}

	if outputDir != "" {
		return filepath.Join(outputDir, filename), nil
	}

	// Default: alongside the original.
	return filepath.Join(filepath.Dir(inputPath), filename), nil
}

func isSupportedFormat(fmt string) bool {
	for _, f := range SupportedFormats {
		if f == fmt {
			return true
		}
	}
	return false
}

// normaliseFormat treats jpg and jpeg as the same canonical format.
func normaliseFormat(f string) string {
	if f == "jpg" {
		return "jpeg"
	}
	return f
}

func confirmBatch(n int, toFmt string) bool {
	fmt.Printf("  Convert %d image(s) to %s? [Y/n] ", n, toFmt)
	var input string
	fmt.Scanln(&input)
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "" || input == "y" || input == "yes"
}

func printDryRun(jobs []job, skipped []SkippedFile) {
	ui.Blank()
	for _, j := range jobs {
		fmt.Printf("  %s  %-30s %s  %s\n",
			ui.StyleMuted.Render("~"),
			filepath.Base(j.input),
			ui.StyleMuted.Render("→"),
			filepath.Base(j.output),
		)
	}
	for _, s := range skipped {
		fmt.Printf("  %s  %-30s %s\n",
			ui.StyleWarning.Render("⊘"),
			filepath.Base(s.Path),
			ui.StyleMuted.Render("skipped  ("+s.Reason+")"),
		)
	}
	ui.Blank()
	ui.Info(fmt.Sprintf("%d file(s) would be converted — dry run, nothing written.", len(jobs)))
}

// ---- Info output renderers -----------------------------------------------

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
	printInfoBasic(info)

	if info.BitDepth > 0 {
		fmt.Printf("  %s %d-bit\n", ui.StyleLabel.Render("Bit depth"), info.BitDepth)
	}

	if info.Raw == nil {
		return
	}

	if rawTags, ok := info.Raw["_tags"]; ok {
		if tags, ok := rawTags.(map[string]string); ok && len(tags) > 0 {
			ui.Blank()
			fmt.Printf("  %s\n", ui.StyleBold.Render("Metadata"))

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

func promptInstallFFmpeg(_ bool, _ string) error {
	ui.Blank()
	ui.Error(
		"ffmpeg is required for image operations",
		"ffmpeg was not found on this system",
		"run `unicli setup` to install it automatically",
	)
	ui.Blank()

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
