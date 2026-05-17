// Copyright © 2026 Prashant Singh
package download

import (
	"bufio"
	"context"
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

	if req.Verbose {
		ui.Muted(fmt.Sprintf("platform: %d  engine: %s", detected.Platform, detected.RecommendEngine))
	}

	engine, err := resolveEngine(detected.RecommendEngine, req.Verbose)
	if err != nil {
		return err
	}

	if err := engine.CanHandle(req.URL); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
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

	if req.Quiet {
		return engine.Download(ctx, engineReq, (&QuietProgress{}).Update)
	}

	return runWithProgressBar(ctx, engine, engineReq, outputDir)
}

// runWithProgressBar manages the progress bar lifecycle.
//
// The bar is created only once we have a real TotalBytes value.
// Before that, a static "Fetching…" line is printed so the terminal
// doesn't look frozen during yt-dlp's metadata extraction phase.
//
// Why not create the bar immediately on the Destination: line?
// That line carries TotalBytes=0. An mpb bar with total=0 is in
// indeterminate mode — it spins but shows no fill. When SetTotal is
// called later with the real size, mpb does switch modes, but the
// first EwmaIncrInt64 call receives a huge initial increment (all bytes
// downloaded since the Destination line) which corrupts the EWMA
// speed/ETA calculation. Creating the bar with the correct total from
// the start avoids both problems.
func runWithProgressBar(
	ctx context.Context,
	engine dlengines.Engine,
	engineReq dlengines.DownloadRequest,
	outputDir string,
) error {
	var pb *ProgressBar
	var savedFilename string
	fetchingPrinted := false

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
			if u.TotalBytes > 0 {
				// Real total known — clear the Fetching line and start the bar.
				if fetchingPrinted {
					fmt.Print("\r\033[K")
				}
				pb = NewProgressBar(savedFilename, u.TotalBytes)
				pb.Update(u)
			} else {
				// No size yet — show a static placeholder.
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

	if savedFilename != "" {
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
