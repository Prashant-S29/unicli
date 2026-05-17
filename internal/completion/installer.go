// Copyright © 2026 Prashant Singh
package completion

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prashant-s29/unicli/internal/ui"
)

type InstallOptions struct {
	Shell   string // explicit override — empty means auto-detect
	Verbose bool
	Yes     bool
}

// Install detects (or uses the provided) shell and installs the completion
// script to the correct location for that shell. Safe to call multiple times.
func Install(opts InstallOptions) error {
	shell := opts.Shell

	if shell == "" {
		shell = detectShell()
		if shell == "" {
			return fmt.Errorf("could not detect shell — use --shell bash|zsh|fish to specify")
		}
		if opts.Verbose {
			ui.Info("Detected shell: " + shell)
		}
	}

	shell = strings.ToLower(strings.TrimSpace(shell))

	switch shell {
	case "zsh":
		return installZsh(opts)
	case "bash":
		return installBash(opts)
	case "fish":
		return installFish(opts)
	default:
		return fmt.Errorf("unsupported shell %q — supported: bash, zsh, fish", shell)
	}
}

// installZsh writes _unicli to ~/.zsh/completions/ and ensures
// fpath and compinit are set in ~/.zshrc — only adds what is missing.
func installZsh(opts InstallOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}

	// 1. Write completion file
	compDir := filepath.Join(home, ".zsh", "completions")
	compFile := filepath.Join(compDir, "_unicli")

	if err := os.MkdirAll(compDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", compDir, err)
	}

	var buf bytes.Buffer
	if err := generateZsh(&buf); err != nil {
		return fmt.Errorf("could not generate zsh completion script: %w", err)
	}

	if err := os.WriteFile(compFile, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", compFile, err)
	}

	if opts.Verbose {
		ui.Info("Wrote " + compFile)
	}

	// 2. Patch ~/.zshrc — add fpath and compinit only if not already present
	zshrc := filepath.Join(home, ".zshrc")
	existing, err := os.ReadFile(zshrc)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("could not read ~/.zshrc: %w", err)
	}

	content := string(existing)
	var additions []string

	fpathLine := `fpath=(~/.zsh/completions $fpath)`
	compInitLine := `autoload -Uz compinit && compinit`

	if !strings.Contains(content, fpathLine) {
		additions = append(additions, fpathLine)
	}
	if !strings.Contains(content, "compinit") {
		additions = append(additions, compInitLine)
	}

	if len(additions) > 0 {
		f, err := os.OpenFile(zshrc, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("could not open ~/.zshrc: %w", err)
		}
		defer f.Close()

		block := "\n# unicli shell completion\n" + strings.Join(additions, "\n") + "\n"
		if _, err := fmt.Fprint(f, block); err != nil {
			return fmt.Errorf("could not write to ~/.zshrc: %w", err)
		}
	}

	ui.Success("Completion installed for zsh")
	ui.Info("Restart your shell or run:  source ~/.zshrc")
	return nil
}

// installBash writes the completion script to
// ~/.local/share/bash-completion/completions/unicli
// This directory is auto-loaded by bash-completion v2 with zero rc edits.
func installBash(opts InstallOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}

	compDir := filepath.Join(home, ".local", "share", "bash-completion", "completions")
	compFile := filepath.Join(compDir, "unicli")

	if err := os.MkdirAll(compDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", compDir, err)
	}

	var buf bytes.Buffer
	if err := generateBash(&buf); err != nil {
		return fmt.Errorf("could not generate bash completion script: %w", err)
	}

	if err := os.WriteFile(compFile, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", compFile, err)
	}

	ui.Success("Completion installed for bash")
	ui.Info("Restart your shell or run:  source " + compFile)
	return nil
}

// installFish writes the completion script to
// ~/.config/fish/completions/unicli.fish
// Fish loads this directory automatically — no config edits needed.
func installFish(opts InstallOptions) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}

	compDir := filepath.Join(home, ".config", "fish", "completions")
	compFile := filepath.Join(compDir, "unicli.fish")

	if err := os.MkdirAll(compDir, 0o755); err != nil {
		return fmt.Errorf("could not create %s: %w", compDir, err)
	}

	var buf bytes.Buffer
	if err := generateFish(&buf); err != nil {
		return fmt.Errorf("could not generate fish completion script: %w", err)
	}

	if err := os.WriteFile(compFile, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("could not write %s: %w", compFile, err)
	}

	ui.Success("Completion installed for fish")
	ui.Info("Completions are active in all new fish sessions — no restart needed")
	return nil
}

// detectShell reads $SHELL and returns the base name (bash, zsh, fish).
func detectShell() string {
	s := os.Getenv("SHELL")
	if s == "" {
		return ""
	}
	base := filepath.Base(s)
	switch base {
	case "bash", "zsh", "fish":
		return base
	default:
		return ""
	}
}
