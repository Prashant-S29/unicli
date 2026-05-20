package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/prashant-s29/unicli/internal/completion"
	"github.com/prashant-s29/unicli/internal/config"
	"github.com/prashant-s29/unicli/internal/engines"
	"github.com/prashant-s29/unicli/internal/ui"
)

// Options controls setup behaviour — mirrors the flags available on the
// `unicli setup` command.
type Options struct {
	Update  bool
	Yes     bool
	Verbose bool
}

// Run is the single entry point for all setup logic.
func Run(opts Options) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("could not load config: %w", err)
	}

	binDir := cfg.Engines.BinDir
	allEngines := engines.All()

	missing, installed := partitionEngines(allEngines, binDir)

	switch {
	case opts.Update:
		return runUpdateFlow(allEngines, binDir, opts.Verbose)
	case len(installed) == 0:
		return runFirstTimeFlow(allEngines, binDir, opts)
	case len(missing) > 0:
		return runInstallMissingFlow(missing, binDir)
	default:
		runStatusCheck(allEngines, binDir)
		return nil
	}
}

// ---- First-time flow -----------------------------------------------------

func runFirstTimeFlow(allEngines []engines.EngineInfo, binDir string, opts Options) error {
	ui.Blank()
	fmt.Println("  Welcome to unicli!")
	ui.Blank()
	fmt.Println("  unicli needs a few dependencies to work.")
	fmt.Printf("  The following will be downloaded and saved to %s\n", binDir)
	ui.Blank()

	for _, e := range allEngines {
		fmt.Printf("    %s %s\n",
			ui.StyleInfo.Render(ui.SymbolDot),
			ui.StyleBold.Render(e.Name))
		ui.Muted("      " + e.Description)
	}

	ui.Blank()

	if !opts.Yes {
		fmt.Print("  Press Enter to continue, or Ctrl+C to cancel. ")
		reader := bufio.NewReader(os.Stdin)
		_, _ = reader.ReadString('\n')
	}

	ui.Blank()

	if err := installAll(allEngines, binDir); err != nil {
		return err
	}

	// Initialise config file
	fmt.Printf("  %-44s", "Creating ~/.unicli/config.yaml...")
	if err := config.Init(); err != nil {
		fmt.Println(ui.StyleError.Render("✗"))
		return fmt.Errorf("could not create config: %w", err)
	}
	fmt.Println(ui.StyleSuccess.Render("done  ✓"))

	// Install shell completions — best effort, never fails setup
	fmt.Printf("  %-44s", "Installing shell completions...")
	err := completion.Install(completion.InstallOptions{
		Verbose: opts.Verbose,
		Yes:     opts.Yes,
	})
	if err != nil {
		fmt.Println(ui.StyleWarning.Render("skipped"))
		ui.Muted("  " + err.Error())
	} else {
		fmt.Println(ui.StyleSuccess.Render("done  ✓"))
	}

	ui.Blank()
	fmt.Printf("  All set. Run %s to get started.\n",
		ui.StyleInfo.Render("`unicli download <url>`"))
	ui.Blank()

	return nil
}

// ---- Install-missing flow ------------------------------------------------

func runInstallMissingFlow(missing []engines.EngineInfo, binDir string) error {
	ui.Blank()
	fmt.Println("  Installing missing engines...")
	ui.Blank()

	if err := installAll(missing, binDir); err != nil {
		return err
	}

	ui.Blank()
	fmt.Println("  All engines ready.")
	ui.Blank()

	return nil
}

// ---- Update flow ---------------------------------------------------------

func runUpdateFlow(allEngines []engines.EngineInfo, binDir string, verbose bool) error {
	ui.Blank()

	for _, e := range allEngines {
		installed, err := engines.InstalledVersion(e.Name, binDir)
		if err != nil && verbose {
			ui.Warning(fmt.Sprintf("%s: could not read installed version: %v", e.Name, err))
		}

		latest, err := engines.LatestVersion(e.Name)
		if err != nil {
			ui.Error(
				fmt.Sprintf("Could not check latest version of %s", e.Name),
				err.Error(),
				"check your internet connection",
			)
			continue
		}

		if installed != "" && (installed == latest || stripV(installed) == stripV(latest)) {
			fmt.Printf("  Updating %-16s already at latest (%s)\n",
				e.Name+"...", latest)
			continue
		}

		action := "Updating"
		if installed == "" {
			action = "Installing"
		}
		label := fmt.Sprintf("  %s %-16s %s -> %s",
			action, e.Name+"...", installed, latest)
		fmt.Printf("%-52s", label)

		if err := engines.Install(e.Name, binDir, nil); err != nil {
			fmt.Println(ui.StyleError.Render("  ✗"))
			ui.Error("Failed", err.Error(), "run unicli setup again or check your internet connection")
			continue
		}

		fmt.Println(ui.StyleSuccess.Render("  ✓"))
	}

	ui.Blank()
	return nil
}

// ---- Status-check flow ---------------------------------------------------

func runStatusCheck(allEngines []engines.EngineInfo, binDir string) {
	ui.Blank()

	for _, e := range allEngines {
		version, err := engines.InstalledVersion(e.Name, binDir)
		if err != nil || version == "" {
			fmt.Printf("  %s  %-16s %s\n",
				ui.StyleError.Render(ui.SymbolError),
				e.Name,
				ui.StyleMuted.Render("not installed"))
			continue
		}
		fmt.Printf("  %s  %-16s %s\n",
			ui.StyleSuccess.Render(ui.SymbolSuccess),
			e.Name,
			ui.StyleMuted.Render(version))
	}

	ui.Blank()
	fmt.Println("  Everything looks good.")
	ui.Blank()
}

// ---- Shared install loop -------------------------------------------------

func installAll(list []engines.EngineInfo, binDir string) error {
	for _, e := range list {
		if err := installOne(e, binDir); err != nil {
			return err
		}
	}
	return nil
}

func installOne(e engines.EngineInfo, binDir string) error {
	goos, goarch := engines.CurrentPlatform()

	_, err := e.AssetName(goos, goarch)
	if err != nil {
		fmt.Printf("  %-44s", fmt.Sprintf("Downloading %s for %s/%s...", e.Name, goos, goarch))
		fmt.Println(ui.StyleWarning.Render("skipped"))
		ui.Muted("  " + err.Error())
		return nil
	}

	label := fmt.Sprintf("  Downloading %s for %s/%s...", e.Name, goos, goarch)
	fmt.Printf("%-44s", label)

	progress := func(done, total int64) { _, _ = done, total }

	if err := engines.Install(e.Name, binDir, progress); err != nil {
		fmt.Println(ui.StyleError.Render("failed  ✗"))
		return fmt.Errorf("failed to install %s: %w", e.Name, err)
	}

	fmt.Println(ui.StyleSuccess.Render("done  ✓"))
	fmt.Printf("  %-44s", "Verifying checksum...")
	fmt.Println(ui.StyleSuccess.Render("done  ✓"))

	return nil
}

// ---- Helpers -------------------------------------------------------------

func partitionEngines(allEngines []engines.EngineInfo, binDir string) (missing, installed []engines.EngineInfo) {
	for _, e := range allEngines {
		_, managed, err := engines.Resolve(e.Name, binDir)
		if err == nil && managed {
			installed = append(installed, e)
		} else {
			missing = append(missing, e)
		}
	}
	return
}

func stripV(v string) string {
	return strings.TrimPrefix(v, "v")
}
