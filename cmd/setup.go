package cmd

import (
	"github.com/prashant-s29/unicli/internal/setup"
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

var setupUpdate bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Install required dependencies and configure unicli",
	Long: `Download and verify all engine binaries unicli depends on,
install shell completions, and create a default config file.

Safe to re-run at any time — checks versions and skips what is already up to date.

Examples:
  unicli setup             first-time setup
  unicli setup --update    re-download latest versions of all engines`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := setup.Run(setup.Options{
			Update:  setupUpdate,
			Yes:     Yes,
			Verbose: Verbose,
		})
		if err != nil {
			ui.Error("Setup failed", err.Error(), "run unicli setup again or check your internet connection")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&setupUpdate, "update", false, "re-download latest versions of all engines")
}
