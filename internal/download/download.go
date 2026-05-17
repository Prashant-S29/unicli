// Copyright © 2026 Prashant Singh
package download

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	dlengines "github.com/prashant-s29/unicli/internal/download/engines"
	mgr "github.com/prashant-s29/unicli/internal/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// Request is the top-level input to the download orchestrator.
// cmd/download.go builds this from CLI flags and calls Run().
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

// Run is the single entry point for all download logic.
// It detects the platform, picks an engine, runs pre-flight checks,
// then streams the download with a live progress bar.
func Run(req Request) error {
	// Dry-run: show what would happen and exit
	if req.DryRun {
		PrintDryRun(req.URL, req.OutputDir)
		return nil
	}

	// 1. Detect platform → choose engine
	detected := Detect(req.URL)

	engine, err := resolveEngine(detected.RecommendEngine)
	if err != nil {
		return err
	}

	if req.Verbose {
		ui.Muted(fmt.Sprintf("engine: %s  platform: %d", engine.Name(), detected.Platform))
	}

	// 2. Pre-flight: can the engine handle this URL right now?
	if err := engine.CanHandle(req.URL); err != nil {
		return fmt.Errorf("pre-flight check failed: %w", err)
	}

	// 3. Build engine request
	engineReq := dlengines.DownloadRequest{
		URL:       req.URL,
		OutputDir: req.OutputDir,
		Format:    req.Format,
		Quality:   req.Quality,
		AudioOnly: req.AudioOnly,
		NoMeta:    req.NoMeta,
		DryRun:    req.DryRun,
	}

	// 4. Set up context with Ctrl+C cancellation
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

	// 5. Choose progress renderer
	if req.Quiet {
		quiet := &QuietProgress{}
		return engine.Download(ctx, engineReq, quiet.Update)
	}

	// Live progress bar — filename and total come from the first ProgressUpdate
	var pb *ProgressBar

	progressFn := func(u dlengines.ProgressUpdate) {
		if pb == nil && u.Filename != "" {
			// Initialise bar on first update that carries a filename
			pb = NewProgressBar(u.Filename, u.TotalBytes)
		}
		if pb != nil {
			pb.Update(u)
		}
	}

	if err := engine.Download(ctx, engineReq, progressFn); err != nil {
		if pb != nil {
			pb.Abort()
		}
		return err
	}

	return nil
}

// resolveEngine returns the concrete Engine implementation for the given name.
// M4 will add yt-dlp and gallery-dl here.
func resolveEngine(name string) (dlengines.Engine, error) {
	switch name {
	case mgr.EngineHTTP, "": // "" = default
		return dlengines.NewHTTPEngine(), nil
	default:
		return nil, fmt.Errorf("unknown engine %q — is unicli setup complete?", name)
	}
}
