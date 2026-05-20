package cmd

import (
	"github.com/prashant-s29/unicli/internal/selfkill"
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

var selfkillCmd = &cobra.Command{
	Use:   "selfkill",
	Short: "Remove all dependencies and configs installed by unicli setup",
	Long: `Removes everything unicli wrote to your machine during setup:

  • ~/.unicli/          engine binaries and config
  • shell completion scripts (bash, zsh, fish)
  • alias symlink (if one was set)
  • the unicli block added to ~/.zshrc

The unicli binary itself is not removed automatically — the command will
print the exact rm command to run as the final step.

Examples:
  unicli selfkill          prompts for confirmation
  unicli selfkill --yes    skips confirmation prompt`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := selfkill.Run(selfkill.Options{
			Yes:     Yes,
			Verbose: Verbose,
		})
		if err != nil {
			ui.Error("selfkill failed", err.Error(), "")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(selfkillCmd)
}
