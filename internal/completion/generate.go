// Copyright © 2026 Prashant Singh
package completion

import (
	"io"

	"github.com/spf13/cobra"
)

// rootCmdRef is set by cmd/completion.go so the generators
// have access to the root command without an import cycle.
var rootCmdRef *cobra.Command

// SetRootCmd must be called from cmd/completion.go init()
// before any Install or generate call is made.
func SetRootCmd(cmd *cobra.Command) {
	rootCmdRef = cmd
}

func generateZsh(w io.Writer) error {
	return generateZshForName("unicli", w)
}

func generateBash(w io.Writer) error {
	return generateBashForName("unicli", w)
}

func generateFish(w io.Writer) error {
	return generateFishForName("unicli", w)
}

// generateZshForName generates a zsh completion script that responds to name.
func generateZshForName(name string, w io.Writer) error {
	return withName(name, func() error {
		return rootCmdRef.GenZshCompletion(w)
	})
}

// generateBashForName generates a bash completion script that responds to name.
func generateBashForName(name string, w io.Writer) error {
	return withName(name, func() error {
		return rootCmdRef.GenBashCompletion(w)
	})
}

// generateFishForName generates a fish completion script that responds to name.
func generateFishForName(name string, w io.Writer) error {
	return withName(name, func() error {
		return rootCmdRef.GenFishCompletion(w, true)
	})
}

// withName temporarily sets the root command's Use field to name,
// runs fn, then restores the original name.
func withName(name string, fn func() error) error {
	original := rootCmdRef.Use
	rootCmdRef.Use = name
	err := fn()
	rootCmdRef.Use = original
	return err
}
