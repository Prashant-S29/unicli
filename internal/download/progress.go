// Copyright © 2026 Prashant Singh
package download

import (
	"fmt"
	"time"

	"github.com/prashant-s29/unicli/internal/download/engines"
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

// ProgressBar manages a single mpb bar for one download.
// It is created before the download starts and finalised when Done is received.
type ProgressBar struct {
	container *mpb.Progress
	bar       *mpb.Bar
	filename  string
}

// NewProgressBar initialises the mpb container and returns a ProgressBar ready
// to receive ProgressUpdate values. Call Done() after the download finishes.
func NewProgressBar(filename string, total int64) *ProgressBar {
	p := mpb.New(
		mpb.WithWidth(60),
		mpb.WithRefreshRate(120*time.Millisecond),
	)

	// Prepend: "  Downloading  filename.ext"
	nameWidth := 20
	label := truncate(filename, nameWidth)

	bar := p.New(total,
		mpb.BarStyle().
			Lbound(" ").
			Filler(ui.StyleInfo.Render("█")).
			Tip(ui.StyleInfo.Render("█")).
			Padding(ui.StyleMuted.Render("░")).
			Rbound(" "),
		mpb.PrependDecorators(
			decor.Name(
				fmt.Sprintf("  %s  %s",
					ui.StyleMuted.Render("Downloading"),
					ui.StyleBold.Render(label)),
				decor.WCSyncWidthR,
			),
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.Counters(decor.SizeB1024(0), " %.1f / %.1f"),
				ui.StyleSuccess.Render("done ✓"),
			),
			decor.Name("  "),
			decor.OnComplete(
				decor.EwmaSpeed(decor.SizeB1024(0), "↓ %.1f ", 30),
				"",
			),
			decor.OnComplete(
				decor.EwmaETA(decor.ET_STYLE_GO, 30),
				"",
			),
		),
	)

	return &ProgressBar{
		container: p,
		bar:       bar,
		filename:  filename,
	}
}

// Update feeds a ProgressUpdate into the bar.
// Must be called from a single goroutine.
func (pb *ProgressBar) Update(u engines.ProgressUpdate) {
	if u.Done {
		pb.bar.SetTotal(u.DoneBytes, true)
		pb.container.Wait()
		return
	}

	// On first update we may now know the filename and total
	if u.Filename != pb.filename && u.Filename != "" {
		pb.filename = u.Filename
		// Note: mpb doesn't support relabelling after creation, so the
		// initial label stands. In practice filename is known on the first
		// update for HTTP downloads.
	}

	if u.TotalBytes > 0 {
		pb.bar.SetTotal(u.TotalBytes, false)
	}

	increment := u.DoneBytes - pb.bar.Current()
	if increment > 0 {
		pb.bar.EwmaIncrInt64(increment, time.Duration(float64(time.Second)/u.Speed+1))
	}
}

// Abort marks the bar as aborted and waits for the render goroutine to stop.
func (pb *ProgressBar) Abort() {
	pb.bar.Abort(true)
	pb.container.Wait()
}

// ---- Quiet-mode progress (no bar, just final line) -----------------------

// QuietProgress swallows all updates and prints a single line when done.
type QuietProgress struct{}

func (q *QuietProgress) Update(u engines.ProgressUpdate) {
	if u.Done {
		ui.Success(fmt.Sprintf("Downloaded  %s", u.Filename))
	}
}

// ---- Dry-run output ------------------------------------------------------

// PrintDryRun prints what would be downloaded without fetching anything.
func PrintDryRun(url, outputDir string) {
	ui.Blank()
	fmt.Printf("  %s\n", ui.StyleBold.Render("Dry run — nothing will be downloaded"))
	ui.Blank()
	fmt.Printf("  %s %s\n", ui.StyleLabel.Render("URL:"), url)
	if outputDir != "" && outputDir != "." {
		fmt.Printf("  %s %s\n", ui.StyleLabel.Render("Output:"), outputDir)
	}
	ui.Blank()
}

// ---- Helpers -------------------------------------------------------------

// truncate shortens s to maxLen runes, adding "…" if truncated.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
