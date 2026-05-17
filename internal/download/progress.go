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
	lastDone  int64
}

// NewProgressBar initialises the mpb container and returns a ProgressBar ready
// to receive ProgressUpdate values. Call Done() after the download finishes.
//
// Pass total = -1 when the size is unknown — the bar renders as a spinner/scroller
// until SetTotal is called with the real value.
func NewProgressBar(filename string, total int64) *ProgressBar {
	p := mpb.New(
		mpb.WithWidth(50),
		mpb.WithRefreshRate(120*time.Millisecond),
	)

	nameWidth := 20
	label := truncate(filename, nameWidth)

	// Use 0 as the initial total so mpb doesn't choke on -1.
	// We'll call SetTotal as soon as we get a real value.
	initTotal := int64(0)
	if total > 0 {
		initTotal = total
	}

	bar := p.New(initTotal,
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
		// Snap the bar to 100 % and wait for the render loop to finish.
		pb.bar.SetTotal(u.DoneBytes, true)
		pb.container.Wait()
		return
	}

	// Update total as soon as we know it.
	if u.TotalBytes > 0 {
		pb.bar.SetTotal(u.TotalBytes, false)
	}

	// How many new bytes arrived since the last update?
	increment := u.DoneBytes - pb.lastDone
	if increment > 0 {
		pb.lastDone = u.DoneBytes

		// EwmaIncrInt64 needs a meaningful elapsed duration per increment.
		// Guard against zero / negative speed to avoid divide-by-zero or
		// nonsense durations.
		var elapsed time.Duration
		if u.Speed > 0 {
			elapsed = time.Duration(float64(time.Second) * float64(increment) / u.Speed)
		} else {
			elapsed = 100 * time.Millisecond // safe fallback
		}

		pb.bar.EwmaIncrInt64(increment, elapsed)
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
