// Copyright © 2026 Prashant Singh
package download

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	dlengines "github.com/prashant-s29/unicli/internal/download/engines"
	mgr "github.com/prashant-s29/unicli/internal/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// Request is the top-level input to the download orchestrator.
type Request struct {
	URL       string
	OutputDir string
	Format    string
	Quality   string
	AudioOnly bool
	NoMeta    bool
	DryRun    bool
	Quiet     bool
	Verbose   bool
}

const binDir = "~/.unicli/bin"

// Run is the single entry point for all download logic.
func Run(req Request) error {

	if strings.Contains(req.URL, "?") && !strings.Contains(req.URL, "&") {
		ui.Warning("URL may be incomplete — your shell split it on '&'")
		ui.Muted("Wrap the URL in quotes: unicli download \"<full-url>\" -o <dir>")
		ui.Blank()
	}

	if req.DryRun {
		PrintDryRun(req.URL, req.OutputDir)
		return nil
	}

	detected := Detect(req.URL)

	// Fail fast for known-unsupported platforms — no engine to try
	if detected.Platform == PlatformUnsupported {
		msg := UnsupportedMessage(req.URL)
		if msg == "" {
			msg = "this platform is not supported"
		}
		return fmt.Errorf("%s", msg)
	}

	if req.Verbose {
		ui.Muted(fmt.Sprintf("platform: %d  engine: %s  fallback: %s",
			detected.Platform, detected.RecommendEngine, detected.FallbackEngine))
	}

	outputDir := req.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	engineReq := dlengines.DownloadRequest{
		URL:       req.URL,
		OutputDir: outputDir,
		Format:    req.Format,
		Quality:   req.Quality,
		AudioOnly: req.AudioOnly,
		NoMeta:    req.NoMeta,
		DryRun:    req.DryRun,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		ui.Blank()
		ui.Warning("Download interrupted — cleaning up...")
		cancel()
	}()

	// --- Primary engine attempt ---

	primaryEngine, err := resolveEngine(detected.RecommendEngine, req.Verbose)
	if err != nil {
		return err
	}

	if err := primaryEngine.CanHandle(req.URL); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}

	primaryErr := runDownload(ctx, primaryEngine, engineReq, outputDir, req.Quiet)

	// Success — done
	if primaryErr == nil {
		return nil
	}

	// Non-retryable error — surface it immediately
	if !errors.Is(primaryErr, dlengines.ErrNoMedia) {
		return primaryErr
	}

	// ErrNoMedia — try fallback engine if one exists
	if detected.FallbackEngine == "" {
		// No fallback registered — translate ErrNoMedia into a clear message
		return fmt.Errorf("no downloadable media found at this URL")
	}

	if req.Verbose {
		ui.Muted(fmt.Sprintf("%s found no media — trying %s",
			detected.RecommendEngine, detected.FallbackEngine))
	}

	fallbackEngine, err := resolveEngine(detected.FallbackEngine, req.Verbose)
	if err != nil {
		// Fallback engine not available — report primary failure cleanly
		return fmt.Errorf("no downloadable media found at this URL")
	}

	fallbackErr := runDownload(ctx, fallbackEngine, engineReq, outputDir, req.Quiet)

	if fallbackErr == nil {
		return nil
	}

	// Both engines failed — if fallback also returned ErrNoMedia, give a
	// clean message; otherwise surface the fallback error (more specific)
	if errors.Is(fallbackErr, dlengines.ErrNoMedia) {
		return fmt.Errorf("no downloadable media found at this URL")
	}
	return fallbackErr
}

// runDownload dispatches to the quiet or progress-bar path.
func runDownload(
	ctx context.Context,
	engine dlengines.Engine,
	engineReq dlengines.DownloadRequest,
	outputDir string,
	quiet bool,
) error {
	if quiet {
		return engine.Download(ctx, engineReq, (&QuietProgress{}).Update)
	}
	return runWithProgressBar(ctx, engine, engineReq, outputDir)
}

// runWithProgressBar manages the progress bar lifecycle.
//
// Bar creation is deferred until we know TotalBytes (byte-progress mode)
// or the first file arrives (count mode, TotalBytes == -1).
// This avoids the corrupted EWMA speed/ETA that results from creating
// an mpb bar with total=0 and then hitting it with a large initial increment.
func runWithProgressBar(
	ctx context.Context,
	engine dlengines.Engine,
	engineReq dlengines.DownloadRequest,
	outputDir string,
) error {
	var pb *ProgressBar
	var savedFilename string
	fetchingPrinted := false
	isCountMode := false // true once gallery-dl starts emitting per-file updates

	progressFn := func(u dlengines.ProgressUpdate) {
		if u.Filename != "" {
			savedFilename = u.Filename
		}

		if u.Done {
			if pb != nil {
				pb.Update(u)
			}
			return
		}

		if pb == nil {
			switch {
			case u.TotalBytes < 0:
				// Count mode (gallery-dl) — create bar on first file
				isCountMode = true
				if fetchingPrinted {
					fmt.Print("\r\033[K")
				}
				pb = NewProgressBar(savedFilename, -1)
				pb.Update(u)

			case u.TotalBytes > 0:
				// Byte mode — real total known
				if fetchingPrinted {
					fmt.Print("\r\033[K")
				}
				pb = NewProgressBar(savedFilename, u.TotalBytes)
				pb.Update(u)

			default:
				// TotalBytes == 0 — still fetching metadata
				if !fetchingPrinted {
					name := truncate(savedFilename, 20)
					if name == "" {
						name = "…"
					}
					fmt.Printf("  %s  %s  %s",
						ui.StyleMuted.Render("Fetching"),
						ui.StyleBold.Render(name),
						ui.StyleMuted.Render("…"),
					)
					fetchingPrinted = true
				}
			}
			return
		}

		pb.Update(u)
	}

	if err := engine.Download(ctx, engineReq, progressFn); err != nil {
		if pb != nil {
			pb.Abort()
		} else if fetchingPrinted {
			fmt.Println()
		}
		return err
	}

	if isCountMode {
		// gallery-dl: files were already saved as they arrived —
		// print the output directory instead of a single filename
		absDir, err := filepath.Abs(outputDir)
		if err != nil {
			absDir = outputDir
		}
		fmt.Printf("  %s %s\n", ui.StyleLabel.Render("Saved:"), absDir)
	} else if savedFilename != "" {
		absPath, err := filepath.Abs(filepath.Join(outputDir, savedFilename))
		if err != nil {
			absPath = filepath.Join(outputDir, savedFilename)
		}
		fmt.Printf("  %s %s\n", ui.StyleLabel.Render("Saved:"), absPath)
	}

	return nil
}

func resolveEngine(name string, _ bool) (dlengines.Engine, error) {
	expandedBinDir := expandHome(binDir)

	switch name {
	case mgr.EngineHTTP, "":
		return dlengines.NewHTTPEngine(), nil

	case mgr.EngineYtDlp:
		engine, _, _, err := dlengines.NewYtDlpEngine(expandedBinDir)
		if err != nil {
			return nil, fmt.Errorf("could not resolve yt-dlp: %w", err)
		}
		if engine != nil {
			return engine, nil
		}
		if !promptInstall(mgr.EngineYtDlp) {
			return nil, fmt.Errorf("yt-dlp is required — run `unicli setup` to install")
		}
		if err := installWithProgress(mgr.EngineYtDlp, expandedBinDir); err != nil {
			return nil, err
		}
		engine, _, _, err = dlengines.NewYtDlpEngine(expandedBinDir)
		if err != nil || engine == nil {
			return nil, fmt.Errorf("yt-dlp install succeeded but binary not found — try `unicli setup`")
		}
		return engine, nil

	case mgr.EngineGalleryDl:
		engine, _, _, err := dlengines.NewGalleryDlEngine(expandedBinDir)
		if err != nil {
			return nil, fmt.Errorf("could not resolve gallery-dl: %w", err)
		}
		if engine != nil {
			return engine, nil
		}
		if !promptInstall(mgr.EngineGalleryDl) {
			return nil, fmt.Errorf("gallery-dl is required — run `unicli setup` to install")
		}
		if err := installWithProgress(mgr.EngineGalleryDl, expandedBinDir); err != nil {
			return nil, err
		}
		engine, _, _, err = dlengines.NewGalleryDlEngine(expandedBinDir)
		if err != nil || engine == nil {
			return nil, fmt.Errorf("gallery-dl install succeeded but binary not found — try `unicli setup`")
		}
		return engine, nil

	default:
		return nil, fmt.Errorf("unknown engine %q — run `unicli setup` to install all engines", name)
	}
}

func promptInstall(engineName string) bool {
	ui.Blank()
	fmt.Printf("  %s needs %s for this download.\n",
		ui.StyleBold.Render("unicli"),
		ui.StyleBold.Render(engineName),
	)
	fmt.Printf("  Download and install it now? %s ", ui.StyleMuted.Render("[Y/n]"))
	sc := bufio.NewScanner(os.Stdin)
	sc.Scan()
	answer := strings.TrimSpace(strings.ToLower(sc.Text()))
	ui.Blank()
	return answer == "" || answer == "y" || answer == "yes"
}

func installWithProgress(engineName, dir string) error {
	fmt.Printf("  %s %s...\n", ui.StyleMuted.Render("Downloading"), ui.StyleBold.Render(engineName))
	err := mgr.Install(engineName, dir, func(done, total int64) {
		if total > 0 {
			fmt.Printf("\r  %d%%", int(float64(done)/float64(total)*100))
		}
	})
	if err != nil {
		fmt.Println()
		return fmt.Errorf("could not install %s: %w", engineName, err)
	}
	fmt.Printf("\r  %s\n\n", ui.StyleSuccess.Render("done ✓"))
	return nil
}

func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}
