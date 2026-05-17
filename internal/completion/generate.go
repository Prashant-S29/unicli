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
	return rootCmdRef.GenZshCompletion(w)
}

func generateBash(w io.Writer) error {
	return rootCmdRef.GenBashCompletion(w)
}

func generateFish(w io.Writer) error {
	return rootCmdRef.GenFishCompletion(w, true)
}
