// Copyright © 2026 Prashant Singh
package engines

import (
	"bytes"
	"fmt"
	"os/exec"
)

// runVersionCommand runs `<binaryPath> --version` and returns stdout.
// This is the only place in the engines package that shells out — kept
// here so manager.go stays free of os/exec imports.
func runVersionCommand(binaryPath string) (string, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command(binaryPath, "--version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("version check failed: %w — %s", err, stderr.String())
	}

	return stdout.String(), nil
}
