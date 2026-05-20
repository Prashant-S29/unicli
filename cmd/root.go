package cmd

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// Global flag values — read by all subcommands via cmd.Verbose etc.
var (
	Verbose bool
	Quiet   bool
	DryRun  bool
	Yes     bool
)

var rootCmd = &cobra.Command{
	Use:   binaryName(),
	Short: "A fast, modular CLI for downloading and transforming media",
	Long: `unicli - one tool for everything.

Download from any platform, convert, compress, and transform
media files from the comfort of your terminal.

Get started:
  unicli setup          install required dependencies
  unicli download <url> download anything`,

	// Don't print usage on every error — only on bad flags/args
	SilenceUsage: true,

	// Errors are printed by Execute(), not Cobra internally
	SilenceErrors: true,
}

// Execute is called by main.go. It runs the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	// Global persistent flags — available on every subcommand
	rootCmd.PersistentFlags().BoolVarP(&Verbose, "verbose", "v", false, "show detailed output")
	rootCmd.PersistentFlags().BoolVarP(&Quiet, "quiet", "q", false, "suppress output except errors")
	rootCmd.PersistentFlags().BoolVar(&DryRun, "dry-run", false, "show what would happen without executing")
	rootCmd.PersistentFlags().BoolVarP(&Yes, "yes", "y", false, "skip confirmation prompts")
}

// binaryName returns the name the binary was invoked as.
// If the user has set an alias (e.g. "dl"), os.Args[0] will be "dl"
// and Cobra will use that name in help text and usage output.
func binaryName() string {
	if len(os.Args) == 0 {
		return "unicli"
	}
	return filepath.Base(os.Args[0])
}
