// Copyright © 2026 Prashant Singh
package cmd

import (
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

var completionShell string

var completionCmd = &cobra.Command{
	Use:   "completion",
	Short: "Manage shell completion scripts",
	Long: `Install or print shell completion scripts for bash, zsh, or fish.

Examples:
  unicli completion install                  auto-detect shell and install
  unicli completion install --shell zsh      install for a specific shell
  unicli completion zsh                      print script to stdout (manual setup)`,
}

var completionInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install completion script into your shell config",
	Run: func(cmd *cobra.Command, args []string) {
		// M6 will replace this with internal/completion logic
		ui.Info("completion install — coming in M6")
	},
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Print bash completion script to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Info("completion bash — coming in M6")
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Print zsh completion script to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Info("completion zsh — coming in M6")
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Print fish completion script to stdout",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Info("completion fish — coming in M6")
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionInstallCmd)
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)

	completionInstallCmd.Flags().StringVar(&completionShell, "shell", "", "shell to install for (bash, zsh, fish)")
}
