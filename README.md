# unicli

A fast, modular command-line tool for everyday file and media operations.

Download from any platform, convert, compress, and transform - all from your terminal.

---

## Install

### macOS (Homebrew)

```bash
brew install prashant-s29/tap/unicli
unicli setup
```

### macOS / Linux

```bash
curl -sSL https://github.com/prashant-s29/unicli/releases/latest/download/install.sh | sh
unicli setup
```

### Windows

Download the latest binary from [GitHub Releases](https://github.com/prashant-s29/unicli/releases/latest), add it to your `PATH`, then run:

```bash
unicli setup
```

---

## What `unicli setup` does

Downloads and verifies all engine binaries unicli depends on, installs shell completions for your shell, and creates a config file at `~/.unicli/config.yaml`. Safe to re-run at any time.

---

## Commands

### Download

Download anything - YouTube, Instagram, Twitter/X, TikTok, Reddit, Vimeo, direct file URLs, image galleries, and 1000+ other platforms.

```bash
unicli download <url>                      # auto-detect and download
unicli download <url> -o ~/Downloads       # output directory
unicli download <url> --audio-only         # extract audio as mp3
unicli download <url> --quality 1080p      # video quality
unicli download <url> --format mp4         # force output format
unicli download <url> --dry-run            # preview without downloading
```

### Setup

```bash
unicli setup                # first-time setup
unicli setup --update       # update all engines to latest
```

### Autocomplete

```bash
unicli completion install                  # auto-detect shell and install
unicli completion install --shell bash     # install for a specific shell
```

### Alias

Give unicli a shorter name:

```bash
unicli alias set dl         # now `dl download <url>` works identically
unicli alias get            # print current alias
unicli alias reset          # remove alias
```

### Selfkill

Remove everything unicli installed:

```bash
unicli selfkill             # prompts for confirmation
unicli selfkill --yes       # skip confirmation
```

---

## Global flags

```
--verbose, -v     Show detailed output
--quiet, -q       Suppress all output except errors
--dry-run         Show what would happen without executing
--yes, -y         Skip confirmation prompts
```

---

## How it works

unicli is a thin interface layer. Heavy lifting is delegated to best-in-class engines:

| Task | Engine |
|---|---|
| Platform media (YouTube, Instagram, etc.) | yt-dlp |
| Image galleries (Pixiv, DeviantArt, etc.) | gallery-dl |
| Direct file downloads | Go stdlib |

Engines are downloaded and managed automatically by `unicli setup`. You never need to install or update them manually.

---

## Uninstall

```bash
unicli selfkill
```

Then follow the printed instruction to remove the binary.

---

## License

MIT - see [LICENSE](LICENSE)
