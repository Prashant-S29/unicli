# unicli - Architecture & Design Document

> **Version:** 0.1.0-draft
> **Status:** Active - this is the single source of truth for what we are building.
> Last updated: 2026-05-17

---

## Table of Contents

1. [Overview](#1-overview)
2. [Design Principles](#2-design-principles)
3. [Tech Stack](#3-tech-stack)
4. [Repository Structure](#4-repository-structure)
5. [Command Design](#5-command-design)
6. [Setup & Dependency Management](#6-setup--dependency-management)
7. [Download Module](#7-download-module)
8. [Autocomplete](#8-autocomplete)
9. [Configuration](#9-configuration)
10. [Alias System](#10-alias-system)
11. [Selfkill](#11-selfkill)
12. [Image Module](#12-image-module) - WIP
13. [Video Module](#13-video-module) - Coming Soon
14. [Audio Module](#14-audio-module) - Coming Soon
15. [PDF Module](#15-pdf-module) - Coming Soon
16. [Error Handling](#16-error-handling)
17. [Distribution](#17-distribution)

---

## 1. Overview

`unicli` is a fast, modular command-line tool for everyday file and media operations. The primary goals are:

- **One tool, everything** - download from any platform, convert, compress, transform
- **Great UX** - progress feedback, clear errors, shell autocomplete that actually works
- **Performant** - single binary, fast startup, no runtime dependencies for the user
- **Modular codebase** - each domain (download, image, video) is an isolated internal package

The first release covers two things only: `download` and the autocomplete infrastructure. Everything else is designed around this foundation.

---

## 2. Design Principles

### Command grammar
All commands follow a strict **noun → verb → target → flags** pattern:

```
unicli <resource> <action> [target] [flags]
unicli download <url> [flags]
unicli image compress <target> --max 20kb [flags]
```

This is consistent, predictable, and tab-completable at every level.

### Wrap, don't rebuild
We do not reinvent wheels. Heavy lifting is delegated to best-in-class external binaries:

| Concern | Delegated to |
|---|---|
| Platform media download | `yt-dlp` |
| Image gallery download | `gallery-dl` |
| Direct file HTTP download | Go stdlib `net/http` |
| Image/video processing | `ffmpeg` *(future modules)* |

`unicli` is the interface layer - detection, routing, progress UI, and consistent error output. The engines do the actual work.

### Fail loudly and clearly
Errors are never swallowed. Every failure surfaces with:
- What went wrong (human-readable)
- Which part of the system failed (engine, network, filesystem)
- A suggested fix where possible

### No magic defaults that surprise
Destructive operations (overwrite, in-place edit) always require an explicit flag. Recursive operations always show a dry-run summary first unless `--yes` is passed.

---

## 3. Tech Stack

| Layer | Choice | Reason |
|---|---|---|
| **Language** | Go 1.22+ | Fast startup (~5ms), single binary, excellent stdlib, best-in-class CLI ecosystem |
| **CLI framework** | [Cobra](https://github.com/spf13/cobra) | Used by kubectl, gh, hugo. First-class autocomplete generation for bash/zsh/fish/powershell |
| **Config management** | [Viper](https://github.com/spf13/viper) | Pairs with Cobra, handles config file + env vars + flags in one layer |
| **HTTP downloads** | Go stdlib `net/http` | Streaming, progress-aware, no external dependency |
| **yt-dlp wrapper** | `os/exec` | Shell out to yt-dlp binary, stream stdout/stderr |
| **gallery-dl wrapper** | `os/exec` | Same pattern as yt-dlp |
| **Progress UI** | [mpb](https://github.com/vbauerster/mpb) | Multi-bar concurrent progress, clean API |
| **Terminal styling** | [lipgloss](https://github.com/charmbracelet/lipgloss) | Charm.sh's styling library - colors, layout, consistent look |
| **Autocomplete** | Cobra built-in + custom dynamic completions | No daemon needed. Go starts fast enough for TAB to feel instant |
| **Build / release** | [goreleaser](https://goreleaser.com/) | Cross-platform binary builds, GitHub releases, Homebrew tap |

### Why Go over Node.js

The extension/plugin system was the main reason to consider Node (npm as distribution). Without it, Go wins on every axis that matters for a CLI:

- No daemon needed for autocomplete - Go binary starts in ~5ms vs Node's ~200-300ms
- True single binary - users install one file, no node_modules, no runtime
- Native concurrency via goroutines - parallel downloads with no callback complexity
- `goreleaser` makes cross-platform distribution trivial

---

## 4. Repository Structure

```
unicli/
│
├── main.go                      # Entry point - 5 lines, calls cmd.Execute()
├── go.mod
├── go.sum
│
├── cmd/                         # One file per top-level command
│   ├── root.go                  # Root command, global flags (--verbose, --quiet, --dry-run)
│   ├── setup.go                 # `unicli setup` command definition
│   ├── download.go              # `unicli download` command definition
│   ├── completion.go            # `unicli completion` - installs shell completion scripts
│   └── alias.go                 # `unicli alias` command group
│
├── internal/                    # All domain logic - not importable externally
│   │
│   ├── download/                # Download domain
│   │   ├── download.go          # Orchestrator - ties detector + engine + progress together
│   │   ├── detector.go          # URL → Platform detection
│   │   ├── engines/
│   │   │   ├── engine.go        # Engine interface definition
│   │   │   ├── manager.go       # Engine lifecycle - resolve, download, verify, store
│   │   │   ├── ytdlp.go         # yt-dlp engine
│   │   │   ├── gallerydl.go     # gallery-dl engine
│   │   │   └── http.go          # Direct HTTP download engine
│   │   └── progress.go          # Unified progress bar UI for download
│   │
│   ├── setup/
│   │   └── setup.go             # Setup orchestration - calls engine manager, completions, config init
│   │
│   ├── config/
│   │   └── config.go            # Load/save ~/.unicli/config.yaml via Viper
│   │
│   ├── alias/
│   │   └── alias.go             # Read/write alias from config, symlink management
│   │
│   └── ui/
│       ├── styles.go            # lipgloss color palette and shared styles
│       └── messages.go          # Standardised success/error/warning/info output helpers
│
├── completions/                 # Pre-generated static completion scripts (committed)
│   ├── unicli.bash
│   ├── unicli.zsh
│   └── unicli.fish
│
└── ARCHITECTURE.md              # This file
```

### Key conventions

- `cmd/` files only handle command parsing and flag definitions. Zero business logic.
- `cmd/` files call into `internal/` packages for everything else.
- `internal/` packages are domain-isolated. `download` never imports from `image`.
- Shared UI/styling helpers live in `internal/ui/` and are imported by all domains.

---

## 5. Command Design

### Global flags (all commands)

```
--verbose, -v      Show detailed output of what is happening
--quiet, -q        Suppress all output except errors
--dry-run          Show what would happen without executing
--yes, -y          Skip confirmation prompts
--version          Print version and exit
--help, -h         Help for any command or subcommand
```

### Full command map (v0.1 scope highlighted)

```
unicli
├── ★ setup [--update]                ← v0.1
├── ★ download <url> [flags]          ← v0.1
├── ★ completion [bash|zsh|fish|ps]   ← v0.1
├── alias
│   ├── set <name>
│   ├── get
│   └── reset
├── image                             ← coming soon
│   ├── compress
│   ├── convert
│   └── resize
├── video                             ← coming soon
├── audio                             ← coming soon
└── pdf                               ← coming soon
```

---

## 6. Setup & Dependency Management

`unicli` ships as a single binary with zero bundled dependencies. External engines (`yt-dlp`, `gallery-dl`) are downloaded once on first setup and managed by unicli from that point forward.

### 6.1 Command interface

```bash
unicli setup              # first-time setup - download all required engines
unicli setup --update     # re-download latest versions of all engines
```

### 6.2 First-time setup flow

```
$ unicli setup

  Welcome to unicli!

  unicli needs a few dependencies to work.
  The following will be downloaded and saved to ~/.unicli/bin/

    • yt-dlp      (media downloader - YouTube, Instagram, Twitter/X and 1000+ sites)
    • gallery-dl  (image gallery downloader - Pixiv, DeviantArt, Danbooru and more)

  Press Enter to continue, or Ctrl+C to cancel.

> [Enter]

  Downloading yt-dlp for darwin/arm64...     ████████████████  done  ✓
  Verifying checksum...                                         done  ✓

  Downloading gallery-dl for darwin/arm64... ████████████████  done  ✓
  Verifying checksum...                                         done  ✓

  Installing shell completions (zsh)...                         done  ✓
  Creating ~/.unicli/config.yaml...                             done  ✓

  All set. Run `unicli download <url>` to get started.
```

Setup also handles:
- Shell detection and autocomplete script installation (asks, defaults yes)
- Creating `~/.unicli/config.yaml` with sane defaults

### 6.3 Re-running setup

Setup is always safe to re-run. On subsequent runs it checks current vs latest versions:

```
$ unicli setup

  ✓  yt-dlp      2024.11.18  (up to date)
  ✓  gallery-dl  1.27.1      (up to date)

  Everything looks good.
```

```
$ unicli setup --update

  Updating yt-dlp...      2024.11.18 → 2024.12.03  ✓
  Updating gallery-dl...  already at latest         ✓
```

### 6.4 Inline prompt (skipped setup)

If a user runs a command that needs an engine before running `unicli setup`, they are not hard-failed - they are prompted inline and setup runs immediately:

```
$ unicli download https://youtube.com/watch?v=...

  unicli needs yt-dlp for this download.
  Download and install it now? [Y/n]

> [Enter]

  Downloading yt-dlp for darwin/arm64... ████████████████  done  ✓
  Continuing...

  Downloading  My Video Title.mp4
  ████████████████████░░░░░░░░░░  62%  45.2 MB / 73.1 MB  ↓ 3.2 MB/s  ETA 8s
```

### 6.5 Engine manager (`internal/engines/manager.go`)

All engine lifecycle logic lives here:

- **Resolve** - check `~/.unicli/bin/<engine>` first, then `$PATH`, then prompt to install
- **Download** - fetch the correct platform build from the engine's official GitHub releases
- **Verify** - SHA256 checksum verified against the release's published hash before use
- **Store** - saved to `~/.unicli/bin/`, marked executable
- **Path injection** - `~/.unicli/bin/` is prepended to the subprocess `PATH` on every engine call so managed binaries are always preferred over any system version

### 6.6 Supported engines

| Engine | Source | Platforms |
|---|---|---|
| `yt-dlp` | [github.com/yt-dlp/yt-dlp](https://github.com/yt-dlp/yt-dlp/releases) | linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64 |
| `gallery-dl` | [github.com/mikf/gallery-dl](https://github.com/mikf/gallery-dl/releases) | linux/amd64, darwin/amd64, windows/amd64 |

### 6.7 Directory layout after setup

```
~/.unicli/
├── config.yaml          # user config
└── bin/
    ├── yt-dlp           # managed engine binary
    └── gallery-dl       # managed engine binary
```

---

## 7. Download Module

### 7.1 Command interface

```bash
unicli download <url>                        # auto-detect everything, download to cwd
unicli download <url> -o ./downloads         # output directory
unicli download <url> -f mp4                 # force output format
unicli download <url> --quality 1080p        # video quality (where applicable)
unicli download <url> --audio-only           # extract audio only
unicli download <url> --no-metadata          # skip embedding metadata
unicli download <url> --dry-run              # show what would be downloaded, don't fetch
```

### 7.2 Detection → Routing workflow

Every download goes through this pipeline:

```
                        ┌─────────────────────────────────┐
                        │         URL comes in             │
                        └──────────────┬──────────────────┘
                                       │
                        ┌──────────────▼──────────────────┐
                        │         detector.go              │
                        │  pattern match against known     │
                        │  platform URL patterns           │
                        └──────────────┬──────────────────┘
                                       │
              ┌────────────────────────┼─────────────────────────┐
              │                        │                          │
   ┌──────────▼──────────┐  ┌─────────▼──────────┐  ┌──────────▼──────────┐
   │  PlatformMedia       │  │  ImageGallery       │  │  DirectFile         │
   │                      │  │                     │  │                     │
   │  YouTube, Instagram, │  │  Pixiv, DeviantArt, │  │  .zip .pdf .mp4     │
   │  Twitter/X, TikTok,  │  │  Danbooru, etc.     │  │  any raw URL        │
   │  Reddit, Vimeo, 1000+│  │                     │  │                     │
   └──────────┬──────────┘  └─────────┬──────────┘  └──────────┬──────────┘
              │                        │                          │
   ┌──────────▼──────────┐  ┌─────────▼──────────┐  ┌──────────▼──────────┐
   │   ytdlp.go engine   │  │ gallerydl.go engine │  │   http.go engine    │
   └──────────┬──────────┘  └─────────┬──────────┘  └──────────┬──────────┘
              │                        │                          │
              └────────────────────────┼─────────────────────────┘
                                       │
                        ┌──────────────▼──────────────────┐
                        │         progress.go              │
                        │   unified progress bar UI        │
                        │   filename, size, speed, ETA     │
                        └──────────────┬──────────────────┘
                                       │
                        ┌──────────────▼──────────────────┐
                        │         output file(s)           │
                        └─────────────────────────────────┘
```

### 7.3 Platform detection (`detector.go`)

Detection is regex pattern matching against the URL host and path. It returns a `Platform` enum and a recommended `Engine`.

```go
type Platform int

const (
    PlatformYouTube Platform = iota
    PlatformInstagram
    PlatformTwitter
    PlatformTikTok
    PlatformReddit
    PlatformVimeo
    PlatformGallery      // gallery-dl territory
    PlatformDirectFile   // raw HTTP file
    PlatformUnknown      // fall through to yt-dlp as best-effort
)
```

Detection order matters - more specific patterns are matched first:
1. Known gallery platforms → `gallery-dl` engine
2. Known video/media platforms → `yt-dlp` engine
3. URL ends in a known file extension → `http` engine
4. Anything else → try `yt-dlp` as best-effort, fall back to `http`

### 7.4 Engine interface (`engines/engine.go`)

All three engines implement the same interface. The orchestrator doesn't care which engine is running.

```go
type Engine interface {
    // Name returns the engine identifier for logging
    Name() string

    // CanHandle does a quick pre-flight check (binary exists, URL reachable)
    CanHandle(url string) error

    // Download executes the download, streaming progress via the callback
    Download(ctx context.Context, req DownloadRequest, progress ProgressFunc) error
}

type DownloadRequest struct {
    URL         string
    OutputDir   string
    Format      string    // optional forced format
    Quality     string    // optional quality hint
    AudioOnly   bool
    NoMetadata  bool
}

type ProgressFunc func(update ProgressUpdate)

type ProgressUpdate struct {
    Filename    string
    TotalBytes  int64
    DoneBytes   int64
    SpeedBps    float64
    ETA         time.Duration
    Done        bool
}
```

### 7.5 yt-dlp engine (`engines/ytdlp.go`)

- Shells out to the `yt-dlp` binary via `os/exec`
- Binary is resolved via engine manager (see §6.5) - never assumes system PATH alone
- Parses yt-dlp's `--progress-template` JSON output to feed the `ProgressFunc`
- Maps unicli flags to yt-dlp args:

```
--audio-only      →  -x --audio-format mp3
--quality 1080p   →  -f "bestvideo[height<=1080]+bestaudio/best[height<=1080]"
--no-metadata     →  --no-embed-metadata
--format mp4      →  --merge-output-format mp4
```

### 7.6 Direct HTTP engine (`engines/http.go`)

For plain file URLs. Uses Go's stdlib `net/http` with:
- Streaming response body (never loads full file in memory)
- `Content-Length` header read for total size
- Chunked reads feeding the `ProgressFunc`
- Partial download resume via `Range` header if file already exists partially

### 7.7 gallery-dl engine (`engines/gallerydl.go`)

- Same shell-out pattern as yt-dlp
- Handles image gallery platforms that yt-dlp doesn't cover well
- Falls back gracefully to yt-dlp if gallery-dl binary is not found

### 7.8 Progress UI (`progress.go`)

Uses `mpb` for multi-bar rendering. A single download shows:

```
  Downloading  video_title.mp4
  ████████████████████░░░░░░░░░░  62%  45.2 MB / 73.1 MB  ↓ 3.2 MB/s  ETA 8s
```

Multiple concurrent downloads (future) show stacked bars. The progress layer is fully decoupled from engines - it only receives `ProgressUpdate` structs.

### 7.9 Pre-flight checks

Before any download starts, unicli verifies:

1. Required engine binary is available - resolved via engine manager (§6.5), inline install prompt if missing
2. Output directory exists or can be created
3. Sufficient disk space (best-effort, based on `Content-Length` or yt-dlp metadata)
4. URL is reachable (HEAD request with 5s timeout)

---

## 8. Autocomplete

### How it works

Go binaries start in ~5ms. No daemon is needed - the shell spawns the binary on every TAB press and it exits immediately with completions.

Cobra handles the mechanics. We add custom dynamic completion functions for context-sensitive cases.

### Shell completion install

```bash
unicli completion install          # auto-detect shell, install to the right rc file
unicli completion install --shell zsh
unicli completion install --shell bash
unicli completion install --shell fish

# Manual (for users who prefer it)
unicli completion zsh > ~/.zsh/completions/_unicli
```

### Static completions (Cobra built-in)

These work out of the box from Cobra's command tree:

```bash
unicli <TAB>              →  download  completion  alias  --help  --version
unicli download <TAB>     →  --output  --format  --quality  --audio-only  ...
unicli alias <TAB>        →  set  get  reset
```

### Dynamic completions (custom)

Context-aware completions registered per-flag:

```bash
unicli download --format <TAB>   →  mp4  mp3  webm  mkv  m4a  ...
unicli download --quality <TAB>  →  best  1080p  720p  480p  360p
```

### Fig / Warp spec

A `fig.ts` completion spec lives at `completions/fig.ts`. Users with Fig or Warp terminals get inline ghost-text suggestions automatically. This spec is generated once and manually maintained alongside command changes.

---

## 9. Configuration

Config file lives at `~/.unicli/config.yaml`. Managed by Viper.

```yaml
# ~/.unicli/config.yaml

alias: unicli             # current binary alias (default: unicli)

download:
  output_dir: "."         # default output directory
  default_quality: best   # default video quality

engines:
  bin_dir: ~/.unicli/bin  # where managed engine binaries live
  ytdlp_path: ""          # override yt-dlp binary path (skips managed bin)
  gallerydl_path: ""      # override gallery-dl binary path (skips managed bin)
```

Config values are the lowest priority - CLI flags always win over config, config always wins over built-in defaults.

Priority order: `CLI flag > env var > config file > default`

---

## 10. Alias System

`unicli alias set <name>` creates a symlink from `<name>` to the `unicli` binary:

```bash
unicli alias set dl         # now `dl download <url>` works identically
unicli alias get            # prints current alias
unicli alias reset          # removes alias, back to unicli only
```

The binary inspects `os.Args[0]` at startup - it works correctly regardless of what name it was invoked as. Autocomplete scripts are re-generated for the alias name automatically.

---


## 11. Selfkill

`unicli selfkill` removes everything unicli wrote to the machine during setup. It is the clean uninstall path.

### Command interface

```bash
unicli selfkill          # prompts for confirmation
unicli selfkill --yes    # skips confirmation prompt
```

### What gets removed

| Item | Path |
|---|---|
| Engine binaries + config | `~/.unicli/` (entire directory) |
| zsh completion script | `~/.zsh/completions/_unicli` |
| bash completion script | `~/.local/share/bash-completion/completions/unicli` |
| fish completion script | `~/.config/fish/completions/unicli.fish` |
| zshrc block | `# unicli shell completion` block in `~/.zshrc` |
| Alias symlink | `<binary dir>/<alias>` (if an alias was set) |
| Alias completion scripts | Same paths as above but for the alias name |

### What does NOT get removed

The `unicli` binary itself is not removed - a running process cannot reliably delete itself cross-platform. Instead, selfkill prints the exact command to run as its final step:

```
  One last step - remove the unicli binary itself:

    sudo rm /usr/local/bin/unicli
```
### Implementation

- Reads the alias name from `~/.unicli/config.yaml` before deleting the directory
- Shell detection is not needed - selfkill always cleans up all three shell locations regardless of which shell is active
- `$SHELL` is intentionally not consulted here; unlike setup, teardown should be exhaustive


---

## 12. Image Module

### 12.1 Overview

The image module is a modular set of operations on image files. Each operation group
is its own sub-milestone (M9a, M9b, ...). All transform operations are handled by
ffmpeg. Text-to-image creation (M9z) uses ImageMagick as a second managed engine.

Command grammar follows the existing noun → verb → target → flags pattern:

    unicli image <action> [target] [flags]

### 12.2 Supported Formats

ffmpeg-only. No patchy or platform-dependent formats shipped.

    Input / Output: jpeg, jpg, png, webp, bmp, tiff, gif, avif

Any file in an unsupported format encountered during a batch operation is skipped
(not errored). The skip is reported at the end with a reason. The overall operation
is never aborted because of an unsupported format.

### 12.3 Batch behavior (all image commands)

- Target defaults to current directory if omitted
- Accepts: single file, multiple files, directory, glob
- --from <fmt>     Filter by input format. Comma-separated. e.g. --from png,jpg
- --recursive      Also process images in subdirectories. Off by default.
- Batch ops (more than one file) without --yes show a dry-run summary and prompt
- --dry-run        Show what would happen, never execute
- --replace        Overwrite originals in place (destructive, explicit opt-in)
- -o, --output     Output file (single) or directory (batch)
- Default output   Non-destructive - output sits alongside original with new extension

### 12.4 Batch result output format

    ✓  photo.png        →  photo.webp
    ✓  banner.jpg       →  banner.webp
    ⊘  profile.heic     →  skipped  (heic not supported)
    ✗  corrupt.jpg      →  failed   (not a valid image)

    Done. 2 converted, 1 skipped, 1 failed.

### 12.5 Convert (M9a)

    unicli image convert [target] --to <format> [flags]

Converts images from one format to another. target defaults to current directory.

Flags:
  --to <format>     Required. Output format.
  --from <fmt>      Only process files of this type. Comma-separated.
  -o, --output      Output file or directory.
  --replace         Overwrite originals. Conflicts with -o.
  --recursive       Include subdirectories.

Autocomplete:
  --to and --from complete from the supported format list defined in
  detector.go. The completion functions source from the same constant
  used for runtime validation — one source of truth.

Edge cases:
  - Same input and output format: warn and skip that file, continue batch
  - --replace and -o together: hard error before anything runs
  - No images found: info message, exit 0

### 12.6 Info (M9a)

    unicli image info <target> [--all]

Reads and displays image metadata via ffprobe (ships with ffmpeg).
target can be a single file, multiple files, or a directory.

Basic (default):

    unicli image info profile.webp

    File      profile.webp
    Format    WebP
    Size      1920 × 1080
    Filesize  234 KB
    Color     YUV (yuv420p)

Full (--all):

    unicli image info profile.webp --all

Everything ffprobe exposes: format, codec details, color space, pixel format,
bit depth, all EXIF fields (camera make/model, GPS coordinates, timestamp, lens
info, exposure, ISO), creation time, all embedded metadata streams. Formatted
where ffprobe gives structured data, raw where it does not.

Multi-file: per-file blocks displayed sequentially, visually separated.
Unsupported formats: skipped with reason, reported at end.

### 12.7 Planned sub-milestones

| ID   | Operation group              | Status       |
|------|------------------------------|--------------|
| M9a  | convert, info                | Defined      |
| M9b  | compress                     | Coming soon  |
| M9c  | resize, crop                 | Coming soon  |
| M9d  | rotate, flip, strip          | Coming soon  |
| M9z  | image create (ImageMagick)   | Coming soon  |

M9z is intentionally last - it introduces a second managed engine (ImageMagick)
and is generative rather than transformative. Architecture will be defined before
implementation begins.

### 12.8 Repository additions

    cmd/
    └── image.go                  # full image command tree

    internal/
    └── image/
        ├── image.go              # orchestrator
        ├── detector.go           # file type detection and validation
        ├── engines/
        │   ├── engine.go         # Engine interface for image ops
        │   └── ffmpeg.go         # all M9a–M9d operations
        └── progress.go           # batch progress and result reporting

### 12.9 Autocomplete

All image subcommands and their flags are fully tab-completable.

Static (Cobra built-in, works from M9a):
    unicli image <TAB>            →  convert  info
    unicli image convert <TAB>    →  --to  --from  --output  --replace  --recursive
    unicli image info <TAB>       →  --all

Dynamic (registered per flag, M9a):
    unicli image convert --to <TAB>      →  jpeg  png  webp  bmp  tiff  gif  avif
    unicli image convert --from <TAB>    →  jpeg  png  webp  bmp  tiff  gif  avif
    unicli image info --to <TAB>         →  jpeg  png  webp  bmp  tiff  gif  avif

As each sub-milestone lands, its new subcommand and flags are added to the
static tree automatically via Cobra, and dynamic completions are registered
in cmd/image.go alongside the flag definitions. The supported format list
never diverges between runtime and autocomplete — both source from
internal/image/detector.go.

---

## 13. Video Module

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned actions: `compress`, `convert`, `trim`, `extract-audio`
> Engine: `ffmpeg` wrapper

---

## 14. Audio Module

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned actions: `convert`, `trim`, `normalize`
> Engine: `ffmpeg` wrapper

---

## 15. PDF Module

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned actions: `merge`, `split`, `compress`, `to-image`
> Engine: TBD

---

## 16. Error Handling

All errors follow a consistent format in the terminal:

```
✗  Failed to download
   Reason:  yt-dlp could not extract video info - video may be private or geo-blocked
   URL:     https://youtube.com/watch?v=...
   Fix:     Try with --verbose to see full yt-dlp output
```

Internally, errors are wrapped with context at every layer using Go's `fmt.Errorf("context: %w", err)` pattern. The top-level command handler unwraps and formats them for display. Raw stack traces are never shown to users unless `--verbose` is set.

Exit codes:

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | General error |
| `2` | Bad usage (wrong flags, missing args) |
| `3` | Engine not found (yt-dlp/gallery-dl not installed) |
| `4` | Network error |
| `5` | Filesystem error (permissions, disk space) |

---

## 17. Distribution

Built with `goreleaser`. One `goreleaser release` command produces:

- Binaries for `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64`, `windows/amd64`
- GitHub Release with checksums
- Homebrew tap formula (`brew install unicli`)
- Shell completion scripts bundled into the release archive

Users on macOS:
```bash
brew install unicli
unicli setup          # downloads engines, installs completions, creates config
```

Users everywhere else:
```bash
# Download binary from GitHub releases, put on $PATH
unicli setup          # downloads engines, installs completions, creates config
```

`unicli setup` is the single recommended first step after install regardless of platform. It is safe to re-run at any time.
