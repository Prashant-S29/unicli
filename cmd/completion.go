// Copyright © 2026 Prashant Singh
package cmd

import (
	"github.com/prashant-s29/unicli/internal/completion"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		err := completion.Install(completion.InstallOptions{
			Shell:   completionShell,
			Verbose: Verbose,
			Yes:     Yes,
		})
		if err != nil {
			ui.Error("Completion install failed", err.Error(), "try running with --shell bash|zsh|fish")
			return err
		}
		return nil
	},
}

var completionBashCmd = &cobra.Command{
	Use:   "bash",
	Short: "Print bash completion script to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenBashCompletion(cmd.OutOrStdout())
	},
}

var completionZshCmd = &cobra.Command{
	Use:   "zsh",
	Short: "Print zsh completion script to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenZshCompletion(cmd.OutOrStdout())
	},
}

var completionFishCmd = &cobra.Command{
	Use:   "fish",
	Short: "Print fish completion script to stdout",
	RunE: func(cmd *cobra.Command, args []string) error {
		return rootCmd.GenFishCompletion(cmd.OutOrStdout(), true)
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
	completionCmd.AddCommand(completionInstallCmd)
	completionCmd.AddCommand(completionBashCmd)
	completionCmd.AddCommand(completionZshCmd)
	completionCmd.AddCommand(completionFishCmd)

	completionInstallCmd.Flags().StringVar(&completionShell, "shell", "", "shell to install for (bash, zsh, fish)")

	// wire root command so internal/completion generators can access it
	completion.SetRootCmd(rootCmd)
}
