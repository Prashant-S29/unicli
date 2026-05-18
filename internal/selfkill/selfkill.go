// Copyright © 2026 Prashant Singh
package selfkill

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/prashant-s29/unicli/internal/config"
	"github.com/prashant-s29/unicli/internal/ui"
)

type Options struct {
	Yes     bool
	Verbose bool
}

func Run(opts Options) error {
	// Read alias before we delete config
	cfg, err := config.Load()
	aliasName := ""
	if err == nil && cfg.Alias != "" && cfg.Alias != "unicli" {
		aliasName = cfg.Alias
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("could not resolve home directory: %w", err)
	}

	ui.Blank()
	fmt.Println("  This will remove everything unicli installed during setup:")
	ui.Blank()
	fmt.Println("    " + ui.StyleInfo.Render(ui.SymbolDot) + "  ~/.unicli/  (engines + config)")
	fmt.Println("    " + ui.StyleInfo.Render(ui.SymbolDot) + "  shell completion scripts")
	if aliasName != "" {
		fmt.Println("    " + ui.StyleInfo.Render(ui.SymbolDot) + "  alias: " + aliasName)
	}
	ui.Blank()

	if !opts.Yes {
		fmt.Print("  Are you sure? This cannot be undone. [y/N] ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(strings.ToLower(input))
		if input != "y" && input != "yes" {
			ui.Blank()
			fmt.Println("  Aborted.")
			ui.Blank()
			return nil
		}
	}

	ui.Blank()

	// 1. Remove ~/.unicli/
	unicliDir := filepath.Join(home, ".unicli")
	removeDir(unicliDir, "~/.unicli/", opts.Verbose)

	// 2. Remove completion scripts for "unicli" across all shells
	removeCompletions("unicli", home, opts.Verbose)

	// 3. Remove completion scripts + symlink for alias if one was set
	if aliasName != "" {
		removeCompletions(aliasName, home, opts.Verbose)
		removeAlias(aliasName, opts.Verbose)
	}

	// 4. Strip unicli block from ~/.zshrc
	stripZshrc(home, opts.Verbose)

	// 5. Print binary removal instruction
	binaryPath, _ := os.Executable()
	ui.Blank()
	fmt.Println("  " + ui.StyleSuccess.Render(ui.SymbolSuccess) + "  All done.")
	ui.Blank()
	fmt.Println("  One last step — remove the unicli binary itself:")
	ui.Blank()
	fmt.Printf("    %s\n", ui.StyleBold.Render("sudo rm "+binaryPath))
	ui.Blank()

	return nil
}

// removeCompletions deletes the completion script for name from all three
// shell locations that installer.go writes to.
func removeCompletions(name, home string, verbose bool) {
	paths := []string{
		filepath.Join(home, ".zsh", "completions", "_"+name),
		filepath.Join(home, ".local", "share", "bash-completion", "completions", name),
		filepath.Join(home, ".config", "fish", "completions", name+".fish"),
	}
	for _, p := range paths {
		removeFile(p, verbose)
	}
}

// removeAlias removes the symlink that `unicli alias set <name>` created.
// The symlink lives in the same directory as the unicli binary.
func removeAlias(name string, verbose bool) {
	self, err := os.Executable()
	if err != nil {
		return
	}
	symlink := filepath.Join(filepath.Dir(self), name)
	removeFile(symlink, verbose)
}

// stripZshrc removes the block that installZshForName appended to ~/.zshrc.
func stripZshrc(home string, verbose bool) {
	zshrc := filepath.Join(home, ".zshrc")

	data, err := os.ReadFile(zshrc)
	if err != nil {
		if !os.IsNotExist(err) && verbose {
			ui.Warning("could not read ~/.zshrc: " + err.Error())
		}
		return
	}

	original := string(data)

	// The exact block installer.go writes:
	// \n# unicli shell completion\n<line>\n<line>\n
	// We strip every line that belongs to the block.
	var kept []string
	skip := false
	for _, line := range strings.Split(original, "\n") {
		if line == "# unicli shell completion" {
			skip = true
			// Also drop the blank line we prepended
			if len(kept) > 0 && kept[len(kept)-1] == "" {
				kept = kept[:len(kept)-1]
			}
			continue
		}
		if skip && (line == `fpath=(~/.zsh/completions $fpath)` ||
			line == `autoload -Uz compinit && compinit`) {
			continue
		}
		skip = false
		kept = append(kept, line)
	}

	cleaned := strings.Join(kept, "\n")
	if cleaned == original {
		if verbose {
			ui.Info("~/.zshrc: nothing to remove")
		}
		return
	}

	if err := os.WriteFile(zshrc, []byte(cleaned), 0o644); err != nil {
		ui.Warning("could not update ~/.zshrc: " + err.Error())
		return
	}

	printStep("Cleaned ~/.zshrc")
	if verbose {
		ui.Info("Removed unicli completion block from ~/.zshrc")
	}
}

// ---- small helpers -------------------------------------------------------

func removeDir(path, label string, verbose bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if verbose {
			ui.Info(label + ": not found, skipping")
		}
		return
	}
	if err := os.RemoveAll(path); err != nil {
		ui.Warning("could not remove " + label + ": " + err.Error())
		return
	}
	printStep("Removed " + label)
}

func removeFile(path string, verbose bool) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if verbose {
			ui.Info(path + ": not found, skipping")
		}
		return
	}
	if err := os.Remove(path); err != nil {
		ui.Warning("could not remove " + path + ": " + err.Error())
		return
	}
	printStep("Removed " + path)
}

func printStep(msg string) {
	fmt.Println("  " + ui.StyleSuccess.Render(ui.SymbolSuccess) + "  " + msg)
}
