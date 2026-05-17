// Copyright © 2026 Prashant Singh
package cmd

import (
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

	Run: func(cmd *cobra.Command, args []string) {
		// M2 will replace this with internal/setup/setup.go
		ui.Info("setup — coming in M2")
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.Flags().BoolVar(&setupUpdate, "update", false, "re-download latest versions of all engines")
}
