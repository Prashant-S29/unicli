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
type ProgressBar struct {
	container *mpb.Progress
	bar       *mpb.Bar
	filename  string
	lastDone  int64
	firstIncr bool // true until the first real increment is applied
}

// NewProgressBar creates the mpb container and bar.
// total must be > 0 — only call this once you have a real size.
func NewProgressBar(filename string, total int64) *ProgressBar {
	p := mpb.New(
		mpb.WithWidth(50),
		mpb.WithRefreshRate(100*time.Millisecond),
	)

	label := truncate(filename, 20)

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
		firstIncr: true,
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

	if u.TotalBytes > 0 {
		pb.bar.SetTotal(u.TotalBytes, false)
	}

	increment := u.DoneBytes - pb.lastDone
	if increment <= 0 {
		return
	}
	pb.lastDone = u.DoneBytes

	if pb.firstIncr {
		// First increment: use non-EWMA so a large initial jump
		// (e.g. 17 MB at once when the first % line arrives late)
		// doesn't corrupt the EWMA speed/ETA state.
		pb.bar.IncrInt64(increment)
		pb.firstIncr = false
		return
	}

	var elapsed time.Duration
	if u.Speed > 0 {
		elapsed = time.Duration(float64(time.Second) * float64(increment) / u.Speed)
	} else {
		elapsed = 100 * time.Millisecond
	}
	pb.bar.EwmaIncrInt64(increment, elapsed)
}

// Abort marks the bar as aborted and waits for the render goroutine to stop.
func (pb *ProgressBar) Abort() {
	pb.bar.Abort(true)
	pb.container.Wait()
}

// ---- Quiet-mode progress -------------------------------------------------

type QuietProgress struct{}

func (q *QuietProgress) Update(u engines.ProgressUpdate) {
	if u.Done {
		ui.Success(fmt.Sprintf("Downloaded  %s", u.Filename))
	}
}

// ---- Dry-run output ------------------------------------------------------

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

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
}
