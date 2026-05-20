package alias

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/prashant-s29/unicli/internal/completion"
	"github.com/prashant-s29/unicli/internal/config"
	"github.com/prashant-s29/unicli/internal/ui"
)

// validName accepts letters, digits, hyphens, and underscores.
var validName = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// Set creates a symlink <name> → unicli binary in ~/.local/bin/,
// then persists the alias in config and regenerates completions.
func Set(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid alias %q — use only letters, digits, hyphens, and underscores", name)
	}
	if name == "unicli" {
		return fmt.Errorf("%q is already the default name — no alias needed", name)
	}

	binaryPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not resolve binary path: %w", err)
	}
	binaryPath, err = filepath.EvalSymlinks(binaryPath)
	if err != nil {
		return fmt.Errorf("could not resolve symlinks on binary path: %w", err)
	}

	dir, err := symlinkDir()
	if err != nil {
		return err
	}

	symlinkPath := filepath.Join(dir, name)

	// Remove existing symlink at that name if present
	if _, err := os.Lstat(symlinkPath); err == nil {
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("could not remove existing symlink at %s: %w", symlinkPath, err)
		}
	}

	if err := os.Symlink(binaryPath, symlinkPath); err != nil {
		return fmt.Errorf("could not create symlink %s → %s: %w", symlinkPath, binaryPath, err)
	}

	ui.Success(fmt.Sprintf("Symlink created: %s → %s", symlinkPath, binaryPath))

	// Warn if ~/.local/bin is not on $PATH
	if !isOnPath(dir) {
		ui.Warning("~/.local/bin is not on your $PATH")
		ui.Info("Add this to ~/.zshrc or ~/.bashrc:  export PATH=\"$HOME/.local/bin:$PATH\"")
	}

	// Persist alias in config
	if err := config.SetAlias(name); err != nil {
		return fmt.Errorf("symlink created but could not save alias to config: %w", err)
	}

	// Regenerate completions for the new name — non-fatal
	if err := RegenerateCompletions(name); err != nil {
		ui.Warning("Could not regenerate completions: " + err.Error())
	}

	ui.Success(fmt.Sprintf("Alias set — you can now use %q instead of %q", name, "unicli"))
	ui.Info("Open a new terminal session for the alias to take effect")
	return nil
}

// Get returns the current alias from config.
func Get() (string, error) {
	cfg, err := config.Load()
	if err != nil {
		return "", fmt.Errorf("could not load config: %w", err)
	}
	if cfg.Alias == "" {
		return "unicli", nil
	}
	return cfg.Alias, nil
}

// Reset removes the symlink and clears the alias back to "unicli".
func Reset() error {
	current, err := Get()
	if err != nil {
		return err
	}

	if current == "unicli" {
		ui.Info("No alias set — already using \"unicli\"")
		return nil
	}

	dir, err := symlinkDir()
	if err != nil {
		return err
	}

	symlinkPath := filepath.Join(dir, current)

	if _, err := os.Lstat(symlinkPath); err == nil {
		if err := os.Remove(symlinkPath); err != nil {
			return fmt.Errorf("could not remove symlink %s: %w", symlinkPath, err)
		}
		ui.Success("Symlink removed: " + symlinkPath)
	}

	if err := config.ClearAlias(); err != nil {
		return fmt.Errorf("symlink removed but could not clear alias in config: %w", err)
	}

	removeAliasCompletions(current)

	ui.Success("Alias reset — back to \"unicli\"")
	return nil
}

// RegenerateCompletions installs completion scripts named after the alias.
func RegenerateCompletions(name string) error {
	shell := completion.DetectShell()
	if shell == "" {
		return nil
	}
	return completion.InstallForName(name, completion.InstallOptions{})
}

// removeAliasCompletions deletes completion files written for the alias.
func removeAliasCompletions(name string) {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	candidates := []string{
		filepath.Join(home, ".zsh", "completions", "_"+name),
		filepath.Join(home, ".local", "share", "bash-completion", "completions", name),
		filepath.Join(home, ".config", "fish", "completions", name+".fish"),
	}

	for _, p := range candidates {
		if _, err := os.Lstat(p); err == nil {
			_ = os.Remove(p)
		}
	}
}

// symlinkDir returns ~/.local/bin, creating it if needed.
// User-owned — no sudo required.
func symlinkDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not resolve home directory: %w", err)
	}
	dir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("could not create %s: %w", dir, err)
	}
	return dir, nil
}

// isOnPath reports whether dir is present in $PATH.
func isOnPath(dir string) bool {
	home, _ := os.UserHomeDir()
	normalize := func(p string) string {
		return strings.ReplaceAll(p, home, "~")
	}
	target := normalize(dir)
	for _, p := range filepath.SplitList(os.Getenv("PATH")) {
		if normalize(p) == target {
			return true
		}
	}
	return false
}
