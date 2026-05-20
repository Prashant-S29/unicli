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
//
// Two rendering modes:
//
//	Byte mode  (countMode=false): standard fill bar showing bytes done/total,
//	           speed, and ETA. Used by HTTP and yt-dlp engines.
//
//	Count mode (countMode=true):  bar advances one step per downloaded file.
//	           TotalBytes is -1 (unknown), so DoneBytes is repurposed as a
//	           file counter. Used by gallery-dl. Speed/ETA decorators are
//	           hidden because per-file byte speed is meaningless here.
type ProgressBar struct {
	container *mpb.Progress
	bar       *mpb.Bar
	filename  string
	lastDone  int64
	firstIncr bool // true until the first real byte increment (byte mode only)
	countMode bool // true when TotalBytes is unknown — gallery-dl file-count mode
}

// NewProgressBar creates the mpb container and bar.
//
// Pass total > 0 for byte-based progress (HTTP, yt-dlp).
// Pass total <= 0 when total size is unknown upfront (gallery-dl count mode).
func NewProgressBar(filename string, total int64) *ProgressBar {
	p := mpb.New(
		mpb.WithWidth(50),
		mpb.WithRefreshRate(100*time.Millisecond),
	)

	label := truncate(filename, 20)
	countMode := total <= 0

	// Seed with 1 in count mode so mpb has a valid non-zero initial total.
	// Update() calls SetTotal() on every file to keep the bar moving.
	initialTotal := total
	if countMode {
		initialTotal = 1
	}

	// Build decorators conditionally so count mode gets a clean file-count
	// display instead of a confusing bytes counter with speed/ETA.
	appendDecorators := buildAppendDecorators(countMode)

	bar := p.New(initialTotal,
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
		mpb.AppendDecorators(appendDecorators...),
	)

	return &ProgressBar{
		container: p,
		bar:       bar,
		filename:  filename,
		firstIncr: true,
		countMode: countMode,
	}
}

// buildAppendDecorators returns the right set of append decorators for the mode.
func buildAppendDecorators(countMode bool) []decor.Decorator {
	if countMode {
		// Count mode: show "N files" — no speed or ETA (meaningless for galleries)
		return []decor.Decorator{
			decor.OnComplete(
				decor.Any(func(s decor.Statistics) string {
					return fmt.Sprintf("  %d files", s.Current)
				}),
				"  "+ui.StyleSuccess.Render("done ✓"),
			),
		}
	}

	// Byte mode: bytes done/total, speed, ETA
	return []decor.Decorator{
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
	}
}

// Update feeds a ProgressUpdate into the bar.
// Must be called from a single goroutine.
func (pb *ProgressBar) Update(u engines.ProgressUpdate) {
	if u.Done {
		if pb.countMode {
			// Finalise at the true file count
			pb.bar.SetTotal(u.DoneBytes, true)
		} else {
			pb.bar.SetTotal(u.DoneBytes, true)
		}
		pb.container.Wait()
		return
	}

	// ---- Count mode (gallery-dl) ----
	if pb.countMode {
		// Keep total one ahead of current so bar never prematurely completes
		pb.bar.SetTotal(u.DoneBytes+1, false)
		pb.bar.SetCurrent(u.DoneBytes)
		return
	}

	// ---- Byte mode (HTTP, yt-dlp) ----
	if u.TotalBytes > 0 {
		pb.bar.SetTotal(u.TotalBytes, false)
	}

	increment := u.DoneBytes - pb.lastDone
	if increment <= 0 {
		return
	}
	pb.lastDone = u.DoneBytes

	if pb.firstIncr {
		// First increment: skip EWMA so a large initial jump
		// doesn't corrupt speed/ETA state
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

func formatBytes(n int64) string {
	const (
		KiB = 1024
		MiB = 1024 * KiB
		GiB = 1024 * MiB
	)
	switch {
	case n >= GiB:
		return fmt.Sprintf("%.1f GiB", float64(n)/GiB)
	case n >= MiB:
		return fmt.Sprintf("%.1f MiB", float64(n)/MiB)
	case n >= KiB:
		return fmt.Sprintf("%.1f KiB", float64(n)/KiB)
	default:
		return fmt.Sprintf("%d B", n)
	}
}
