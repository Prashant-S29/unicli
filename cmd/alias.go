// Copyright © 2026 Prashant Singh
package cmd

import (
	"github.com/prashant-s29/unicli/internal/ui"
	"github.com/spf13/cobra"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage the unicli invocation alias",
	Long: `Set a custom name for the unicli binary.

Examples:
  unicli alias set dl       invoke unicli as 'dl' from now on
  unicli alias get          show the current alias
  unicli alias reset        remove alias, back to 'unicli' only`,
}

var aliasSetCmd = &cobra.Command{
	Use:   "set <name>",
	Short: "Set a custom alias for unicli",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// M7 will replace this with internal/alias logic
		ui.Info("alias set — coming in M7")
		ui.Muted("name: " + args[0])
	},
}

var aliasGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current alias",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Info("alias get — coming in M7")
	},
}

var aliasResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove alias and return to 'unicli'",
	Run: func(cmd *cobra.Command, args []string) {
		ui.Info("alias reset — coming in M7")
	},
}

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasSetCmd)
	aliasCmd.AddCommand(aliasGetCmd)
	aliasCmd.AddCommand(aliasResetCmd)
}
