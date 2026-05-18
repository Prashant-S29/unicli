// Copyright © 2026 Prashant Singh
package cmd

import (
	"fmt"

	"github.com/prashant-s29/unicli/internal/alias"
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
	RunE: func(cmd *cobra.Command, args []string) error {
		err := alias.Set(args[0])
		if err != nil {
			ui.Error("Failed to set alias", err.Error(), "make sure you have write permission to the binary directory")
			return err
		}
		return nil
	},
}

var aliasGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the current alias",
	RunE: func(cmd *cobra.Command, args []string) error {
		name, err := alias.Get()
		if err != nil {
			ui.Error("Failed to get alias", err.Error(), "run unicli setup to initialise config")
			return err
		}
		fmt.Println(name)
		return nil
	},
}

var aliasResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Remove alias and return to 'unicli'",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := alias.Reset()
		if err != nil {
			ui.Error("Failed to reset alias", err.Error(), "try removing the symlink manually from the binary directory")
			return err
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(aliasCmd)
	aliasCmd.AddCommand(aliasSetCmd)
	aliasCmd.AddCommand(aliasGetCmd)
	aliasCmd.AddCommand(aliasResetCmd)
}
