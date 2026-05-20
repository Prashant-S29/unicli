package image

import (
	"fmt"
	"path/filepath"

	imgengines "github.com/prashant-s29/unicli/internal/image/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// BatchResult accumulates outcomes from a convert run.
type BatchResult struct {
	converted []convertedFile
	skipped   []SkippedFile
	failed    []SkippedFile
}

type convertedFile struct {
	inputPath  string
	outputPath string
}

// RecordConvert is passed as the ProgressFunc to engine.Convert.
// It collects each result into the BatchResult.
func (b *BatchResult) RecordConvert(result imgengines.ConvertResult) {
	if result.Err != nil {
		b.failed = append(b.failed, SkippedFile{
			Path:   result.InputPath,
			Reason: result.Err.Error(),
		})
		return
	}
	b.converted = append(b.converted, convertedFile{
		inputPath:  result.InputPath,
		outputPath: result.OutputPath,
	})
}

// Print renders the full batch summary to stdout.
//
//	✓  photo.png        →  photo.webp
//	⊘  profile.heic     →  skipped  (heic not supported)
//	✗  corrupt.jpg      →  failed   (ffmpeg failed: ...)
//
//	Done. 2 converted, 1 skipped, 1 failed.
func (b *BatchResult) Print() {
	for _, f := range b.converted {
		fmt.Printf("  %s  %-30s %s  %s\n",
			ui.StyleSuccess.Render(ui.SymbolSuccess),
			filepath.Base(f.inputPath),
			ui.StyleMuted.Render("→"),
			filepath.Base(f.outputPath),
		)
	}

	for _, f := range b.skipped {
		fmt.Printf("  %s  %-30s %s\n",
			ui.StyleWarning.Render("⊘"),
			filepath.Base(f.Path),
			ui.StyleMuted.Render("skipped  ("+f.Reason+")"),
		)
	}

	for _, f := range b.failed {
		fmt.Printf("  %s  %-30s %s\n",
			ui.StyleError.Render(ui.SymbolError),
			filepath.Base(f.Path),
			ui.StyleMuted.Render("failed   ("+f.Reason+")"),
		)
	}

	ui.Blank()

	total := len(b.converted) + len(b.skipped) + len(b.failed)
	if total == 0 {
		ui.Info("No images found")
		return
	}

	fmt.Printf("  Done. %s\n", b.summarySentence())
}

// HasErrors returns true if any file failed (not skipped — skips are expected).
func (b *BatchResult) HasErrors() bool {
	return len(b.failed) > 0
}

func (b *BatchResult) summarySentence() string {
	parts := []string{}
	if len(b.converted) > 0 {
		parts = append(parts, fmt.Sprintf("%d converted", len(b.converted)))
	}
	if len(b.skipped) > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", len(b.skipped)))
	}
	if len(b.failed) > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", len(b.failed)))
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	return result + "."
}

// wasConverted returns true if the given input path converted successfully.
func (b *BatchResult) wasConverted(inputPath string) bool {
	for _, f := range b.converted {
		if f.inputPath == inputPath {
			return true
		}
	}
	return false
}
