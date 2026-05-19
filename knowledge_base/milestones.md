# unicli - Milestones

> **Status:** Planning
> Last updated: 2026-05-17
>
> Each milestone is fully independent. Completing M2 does not require changes to M1.
> New milestones only add - they never modify or break what came before.
> Every milestone ends in a working, shippable binary.

---

## Table of Contents

- [M1 - Skeleton](#m1--skeleton)
- [M2 - Setup & Engine Manager](#m2--setup--engine-manager)
- [M3 - Download: Direct HTTP](#m3--download-direct-http)
- [M4 - Download: Platform Media (yt-dlp)](#m4--download-platform-media-yt-dlp)
- [M5 - Download: Image Galleries (gallery-dl)](#m5--download-image-galleries-gallery-dl)
- [M6 - Autocomplete](#m6--autocomplete)
- [M7 - Alias System](#m7--alias-system)
- [M8 - Polish & Hardening](#m8--polish--hardening)
- [M9 - Image Module](#m9--image-module)
- [M10 - Video Module](#m10--video-module)
- [M11 - Audio Module](#m11--audio-module)
- [M12 - PDF Module](#m12--pdf-module)

---

## M1 - Skeleton

> **Goal:** A working Go project that compiles, runs, and has the full command tree registered. No real functionality yet - just the scaffolding everything else plugs into.

### What gets built

- Repo initialized - `go.mod`, `go.sum`, `.gitignore`, `README.md`
- `main.go` - entry point, calls `cmd.Execute()`
- `cmd/root.go` - root Cobra command, global flags wired up (`--verbose`, `--quiet`, `--dry-run`, `--yes`)
- `cmd/setup.go` - command registered, prints `"not implemented"` placeholder
- `cmd/download.go` - command registered, all flags defined, prints `"not implemented"` placeholder
- `cmd/completion.go` - command registered, prints `"not implemented"` placeholder
- `cmd/alias.go` - command group registered with `set`, `get`, `reset` subcommands, all print `"not implemented"`
- `internal/ui/styles.go` - lipgloss palette and base styles defined (used by everything from here on)
- `internal/ui/messages.go` - `Success()`, `Error()`, `Warning()`, `Info()` helpers defined
- `internal/config/config.go` - Viper wired up, `~/.unicli/config.yaml` path set, default values defined

### What you can do after M1

```bash
unicli --help
unicli download --help
unicli setup --help
unicli alias --help
unicli alias set --help
```

Every command responds. Global flags are recognized. Help text is accurate. Binary compiles to a single file.

### What does NOT exist yet

No actual functionality. All commands print a placeholder. No engines, no downloads, no setup logic.

### Contracts established (do not break in later milestones)

- Global flag names and shorthand (`-v`, `-q`, `--dry-run`, `--yes`)
- All top-level command names (`setup`, `download`, `completion`, `alias`)
- `internal/ui` message helper signatures
- `internal/config` config struct shape and file path (`~/.unicli/config.yaml`)

---

## M2 - Setup & Engine Manager

> **Goal:** `unicli setup` fully works. By the end of this milestone, running setup downloads and verifies all required engine binaries onto the user's machine. No download functionality yet - just the engine management layer that download will call.

### Depends on

M1 (skeleton, config, UI helpers)

### What gets built

- `internal/engines/manager.go`
  - `Resolve(engine)` - checks `~/.unicli/bin/` then `$PATH`, returns binary path or triggers install
  - `Download(engine, platform)` - fetches correct release binary from GitHub for current OS/arch
  - `Verify(path, expectedSHA256)` - SHA256 checksum verification before marking executable
  - `Store(binary, destination)` - saves to `~/.unicli/bin/`, sets permissions
  - `CheckVersion(engine)` - reads installed version, compares to latest GitHub release tag
- `internal/setup/setup.go`
  - `Run(updateMode bool)` - orchestrates the full setup flow
  - On first run: downloads all engines, installs completions stub, creates config
  - On re-run: prints version status per engine, skips if up to date
  - On `--update`: re-downloads all engines to latest
- `cmd/setup.go` - wired to `internal/setup`, placeholder removed

### What you can do after M2

```bash
unicli setup
# → shows welcome message, downloads yt-dlp + gallery-dl, verifies checksums

unicli setup
# → re-run: shows "everything up to date"

unicli setup --update
# → re-downloads latest versions of all engines
```

`~/.unicli/bin/yt-dlp` and `~/.unicli/bin/gallery-dl` exist and are executable.

### What does NOT exist yet

No download command. Engines are present on disk but nothing calls them yet.

### Contracts established

- `manager.Resolve(engine) (string, error)` - signature locked
- `~/.unicli/bin/` as the managed binary directory
- Engine names as typed constants (`EngineYtDlp`, `EngineGalleryDl`)
- Platform detection logic (OS + arch → correct binary asset name)

---

## M3 - Download: Direct HTTP

> **Goal:** `unicli download` works for plain file URLs - any direct link to a file (`.zip`, `.pdf`, `.mp4`, `.png`, etc). No platform detection yet, no yt-dlp. Just clean HTTP downloading with a proper progress bar.

### Depends on

M1 (skeleton, config, UI), M2 (engine manager for `Resolve` pattern, even though HTTP needs no engine binary)

### What gets built

- `internal/download/engines/engine.go` - `Engine` interface defined
  ```go
  type Engine interface {
      Name() string
      CanHandle(url string) error
      Download(ctx context.Context, req DownloadRequest, fn ProgressFunc) error
  }
  ```
- `internal/download/engines/http.go` - implements `Engine`
  - Streaming `net/http` download, never loads full file into memory
  - Reads `Content-Length` for total size
  - Chunked reads feeding `ProgressFunc`
  - Resume support via `Range` header if partial file exists
- `internal/download/progress.go` - `mpb` progress bar, consumes `ProgressUpdate` structs
  ```
  Downloading  report.pdf
  ████████████████████░░░░░░░░░░  62%  12.4 MB / 20.1 MB  ↓ 3.2 MB/s  ETA 4s
  ```
- `internal/download/detector.go` - stub: always returns `EngineHTTP` for now (platform detection comes in M4)
- `internal/download/download.go` - orchestrator: detector → engine → progress
- `cmd/download.go` - wired to orchestrator, placeholder removed

### What you can do after M3

```bash
unicli download https://example.com/file.zip
unicli download https://example.com/video.mp4 -o ./videos
unicli download https://example.com/report.pdf --dry-run
```

Progress bar renders. Resume works if connection drops mid-download. Output directory flag works.

### What does NOT exist yet

Platform URLs (YouTube, Instagram, etc.) don't work - they either download the HTML page or fail. That's expected. detector.go is a stub.

### Contracts established

- `Engine` interface - signature locked, all future engines implement this
- `DownloadRequest` struct fields
- `ProgressUpdate` struct fields
- `ProgressFunc` type signature
- Orchestrator call signature in `download.go`

---

## M4 - Download: Platform Media (yt-dlp)

> **Goal:** `unicli download` works for all platforms yt-dlp supports - YouTube, Instagram, Twitter/X, TikTok, Reddit, Vimeo, and 1000+ others. Platform detection is now real.

### Depends on

M1, M2 (engine manager resolves yt-dlp binary), M3 (Engine interface, orchestrator, progress)

### What gets built

- `internal/download/engines/ytdlp.go` - implements `Engine`
  - Resolves binary via `manager.Resolve(EngineYtDlp)` - inline install prompt if missing
  - Runs yt-dlp with `--progress-template` JSON output
  - Parses progress JSON → feeds `ProgressFunc`
  - Flag mapping:
    ```
    --audio-only      →  -x --audio-format mp3
    --quality 1080p   →  -f "bestvideo[height<=1080]+bestaudio/best"
    --no-metadata     →  --no-embed-metadata
    --format mp4      →  --merge-output-format mp4
    ```
- `internal/download/detector.go` - real implementation
  - Regex pattern matching against known platform URL patterns
  - Returns `Platform` enum + recommended engine
  - Fallback chain: known gallery → yt-dlp → HTTP
- Pre-flight checks in orchestrator:
  - URL reachability (HEAD request, 5s timeout)
  - Output directory writable
  - Disk space best-effort check

### What you can do after M4

```bash
unicli download https://youtube.com/watch?v=...
unicli download https://twitter.com/user/status/...
unicli download https://instagram.com/p/...
unicli download https://reddit.com/r/.../comments/...
unicli download https://vimeo.com/...

unicli download https://youtube.com/watch?v=... --audio-only
unicli download https://youtube.com/watch?v=... --quality 720p
unicli download https://youtube.com/watch?v=... --format mp4 -o ~/Downloads
unicli download https://youtube.com/watch?v=... --dry-run
```

Direct file URLs from M3 still work - detector routes them to the HTTP engine.

### Contracts established

- `Platform` enum values and their string representations
- Detector return type and fallback order

---

## M5 - Download: Image Galleries (gallery-dl)

> **Goal:** `unicli download` works for image gallery platforms - Pixiv, DeviantArt, Danbooru, and others that yt-dlp doesn't cover well.

### Depends on

M1, M2 (engine manager resolves gallery-dl binary), M3 (Engine interface), M4 (real detector)

### What gets built

- `internal/download/engines/gallerydl.go` - implements `Engine`
  - Resolves binary via `manager.Resolve(EngineGalleryDl)`
  - Inline install prompt if missing (same pattern as yt-dlp)
  - Parses gallery-dl progress output → feeds `ProgressFunc`
  - Graceful fallback to yt-dlp if gallery-dl binary unavailable
- `internal/download/detector.go` - updated with gallery platform patterns
  - gallery-dl platforms checked before yt-dlp in fallback chain

### What you can do after M5

```bash
unicli download https://pixiv.net/artworks/...
unicli download https://deviantart.com/user/art/...
```

All M3 and M4 functionality unchanged.

---

## M6 - Autocomplete

> **Goal:** Tab completion works in bash, zsh, and fish. Both static (command/flag names) and dynamic (format values, quality values) completions are in place.

### Depends on

M1 (full command tree must be registered for Cobra to generate completions)

### What gets built

- `cmd/completion.go` - real implementation
  - `unicli completion install` - detects shell, appends source line to rc file
  - `unicli completion install --shell zsh|bash|fish` - explicit shell override
  - `unicli completion zsh|bash|fish` - print raw script to stdout (for manual setup)
- Dynamic completion functions registered on `cmd/download.go` flags:
  - `--format <TAB>` → `mp4 mp3 webm mkv m4a flac wav`
  - `--quality <TAB>` → `best 1080p 720p 480p 360p 240p`
- Pre-generated static scripts committed to `completions/`
  - `completions/unicli.bash`
  - `completions/unicli.zsh`
  - `completions/unicli.fish`
- `completions/fig.ts` - Fig/Warp spec for inline ghost-text suggestions

### What you can do after M6

```bash
unicli completion install       # one-time, then restart shell

unicli <TAB>                    # → setup  download  completion  alias
unicli download <TAB>           # → --output  --format  --quality  --audio-only ...
unicli download --format <TAB>  # → mp4  mp3  webm  mkv  m4a ...
unicli download --quality <TAB> # → best  1080p  720p  480p ...
unicli alias <TAB>              # → set  get  reset
```

All download functionality from M3–M5 unchanged.

---

## M7 - Alias System

> **Goal:** Users can give `unicli` a custom name that works identically.

### Depends on

M1 (config, root command), M6 (so autocomplete is regenerated for the alias)

### What gets built

- `internal/alias/alias.go`
  - `Set(name)` - creates symlink `<name> → unicli` in the same directory as the binary, updates config
  - `Get()` - reads current alias from config
  - `Reset()` - removes symlink, clears alias in config
  - `RegenerateCompletions(name)` - re-runs completion install for the new name
- `cmd/alias.go` - wired to `internal/alias`, placeholder removed
- `main.go` / `cmd/root.go` - `os.Args[0]` inspection so binary responds correctly regardless of invocation name

### What you can do after M7

```bash
unicli alias set dl
dl download https://youtube.com/...   # works identically
dl alias get                          # → dl
dl alias reset                        # back to unicli only
```

Tab completion works for the alias name too.

---

## M8 - Polish & Hardening

> **Goal:** Everything built so far is robust, well-tested, and ready for real users. No new features - only quality.

### Depends on

M1–M7 (all prior milestones)

### What gets built

- **Error messages** - every error path reviewed for clarity. Format:
  ```
  ✗  Failed to download
     Reason:  video is private or geo-blocked
     Fix:     try with --verbose to see full output
  ```
- **Exit codes** - all 6 exit codes verified correct across all error paths
- **`--dry-run`** - verified working on all commands that support it
- **`--verbose`** - full engine output streamed to stderr when set
- **`--quiet`** - no output except errors when set
- **Unit tests** - `detector.go` URL pattern matching, `manager.go` platform detection, `config.go` read/write
- **Integration smoke tests** - one test per engine that runs against a known stable public URL
- **`unicli setup` re-run** - all edge cases handled (partial install, corrupt binary, checksum mismatch)
- **Disk space handling** - clear error when not enough space before download starts
- **Interrupt handling** - `Ctrl+C` cleans up partial files gracefully
- **README.md** - install instructions, quick start, all commands documented

### What you can do after M8

Everything from M1–M7 works reliably. The binary is ready for a public v1.0.0 release.

---

## M9 - Image Module

### M9a: Convert & Info

> **Goal:** `unicli image convert` and `unicli image info` fully work.
> First milestone of the image module. Establishes the engine interface and
> internal structure that all subsequent image milestones (M9b onward) plug into.

### Depends on

M1 (skeleton, config, UI), M2 (engine manager - resolves and manages ffmpeg binary)

### What gets built

- `internal/engines/manager.go` - extended with `EngineFFmpeg` constant and
  platform binary asset map (linux/amd64, linux/arm64, darwin/amd64,
  darwin/arm64, windows/amd64)
- `internal/image/engines/engine.go` - ImageEngine interface:
```go
  type ImageEngine interface {
      Name() string
      CanHandle(file string) error
      Convert(ctx context.Context, req ConvertRequest, fn ProgressFunc) error
      Info(ctx context.Context, file string, full bool) (*ImageInfo, error)
  }
```
- `internal/image/engines/ffmpeg.go` - implements ImageEngine
  - Convert: shells out to ffmpeg, one subprocess per file
  - Info: shells out to ffprobe with -v quiet -print_format json -show_streams
    -show_format, parses JSON into ImageInfo struct
  - Resolves binary via engine manager - inline install prompt if missing
- `internal/image/detector.go`
  - Enumerates files from target (single file / multi-file / directory / glob)
  - Filters by supported format list
  - Returns supported files + skipped files (with reason) before any processing
- `internal/image/image.go` - orchestrator
  - Calls detector, runs pre-flight (output dir writable, no --replace + -o conflict)
  - Batch confirmation prompt unless --yes
  - Dispatches to engine per file, collects results
  - Renders final summary
- `internal/image/progress.go`
  - Per-file status lines (✓ / ⊘ / ✗)
  - Final summary line: N converted, N skipped, N failed
- `cmd/image.go` - image command group with convert and info subcommands,
  all flags wired, no placeholder
- `cmd/image.go` — dynamic completion functions registered on all flags:
  - `--to <TAB>`     →  `jpeg png webp bmp tiff gif avif`
  - `--from <TAB>`   →  `jpeg png webp bmp tiff gif avif`
  - `image <TAB>`    →  `convert info` (expands as M9b+ land)

### What you can do after M9a

```bash
# Convert
unicli image convert photo.png --to webp
unicli image convert --to webp                          # all images in cwd
unicli image convert ./assets --to webp
unicli image convert ./assets --from png --to webp
unicli image convert ./assets --from png,jpg --to webp
unicli image convert ./assets --to webp --recursive
unicli image convert ./assets --to webp -o ./out
unicli image convert ./assets --to webp --replace
unicli image convert ./assets --to webp --dry-run

# Info
unicli image info photo.png
unicli image info photo.png --all
unicli image info ./assets
unicli image info ./assets --all
```

### What does NOT exist yet

compress, resize, crop, rotate, flip, strip - all coming in M9b onward.

### Contracts established (do not break in later milestones)

- `ImageEngine` interface - signature locked
- `ConvertRequest` and `ImageInfo` struct fields
- `ProgressFunc` type for image ops
- `detector.go` return types (supported list + skip list)
- `cmd/image.go` command and flag names
- `EngineFFmpeg` constant and manager integration
- Supported format list as a typed constant slice in `internal/image/detector.go`
  this is the single source of truth for both runtime validation AND
  autocomplete completion functions. Adding a format in one place automatically
  covers both. Later milestones (M9b, M9c, M9d) must source from this same
  constant, never hardcode format strings in their own completion functions.

---

## M9b - Image Module: Compress

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned commands: `unicli image compress`
> Engine: ffmpeg
>
> M9b adds to `internal/image/` and `cmd/image.go`. Nothing in M9a is modified.

---

## M9c - Image Module: Resize & Crop

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned commands: `unicli image resize`, `unicli image crop`
> Engine: ffmpeg
>
> M9c adds to `internal/image/` and `cmd/image.go`. Nothing in M9a–M9b is modified.

---

## M9d - Image Module: Rotate, Flip & Strip

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned commands: `unicli image rotate`, `unicli image flip`, `unicli image strip`
> Engine: ffmpeg
>
> M9d adds to `internal/image/` and `cmd/image.go`. Nothing in M9a–M9c is modified.

---

## M9z - Image Module: Create

> **Coming Soon.** Architecture will be defined before implementation begins.
>
> Planned commands: `unicli image create`
> Engine: ImageMagick (second managed engine, added here)
>
> Generates an image from text with configurable canvas, typography, alignment,
> background and foreground color, padding, and output format.
>
> M9z adds `internal/image/engines/imagemagick.go` and extends `cmd/image.go`.
> Nothing in M9a–M9d is modified.

---

## M10 - Video Module

> **Coming Soon.**
>
> Architecture will be defined in full before implementation begins.
>
> Planned commands: `unicli video compress`, `unicli video convert`, `unicli video trim`, `unicli video extract-audio`
> Engine: `ffmpeg`
>
> M10 adds `cmd/video.go` and `internal/video/`. Nothing in M1–M9 is modified.

---

## M11 - Audio Module

> **Coming Soon.**
>
> Architecture will be defined in full before implementation begins.
>
> Planned commands: `unicli audio convert`, `unicli audio trim`, `unicli audio normalize`
> Engine: `ffmpeg`
>
> M11 adds `cmd/audio.go` and `internal/audio/`. Nothing in M1–M10 is modified.

---

## M12 - PDF Module

> **Coming Soon.**
>
> Architecture will be defined in full before implementation begins.
>
> Planned commands: `unicli pdf merge`, `unicli pdf split`, `unicli pdf compress`, `unicli pdf to-image`
> Engine: TBD
>
> M12 adds `cmd/pdf.go` and `internal/pdf/`. Nothing in M1–M11 is modified.

---

## Milestone Summary

| Milestone | Builds | Depends on | Ships |
|---|---|---|---|
| **M1** | Skeleton, CLI tree, config, UI helpers | - | Binary that responds to all commands |
| **M2** | Setup command, engine manager, binary download + verify | M1 | `unicli setup` fully working |
| **M3** | HTTP download engine, progress bar, orchestrator | M1, M2 | Direct file URL downloads |
| **M4** | yt-dlp engine, real URL detector | M1, M2, M3 | Platform media downloads (1000+ sites) |
| **M5** | gallery-dl engine, detector updated | M1, M2, M3, M4 | Image gallery downloads |
| **M6** | Shell autocomplete, Fig spec | M1 | Tab completion in bash/zsh/fish |
| **M7** | Alias system | M1, M6 | Custom binary name |
| **M8** | Tests, error hardening, README, polish | M1–M7 | Public v1.0.0 release |
| **M9a** | Image: convert, info            | M1, M2      | `unicli image convert` + `unicli image info` |
| **M9b** | Image: compress                 | M1, M2, M9a | `unicli image compress`                      |
| **M9c** | Image: resize, crop             | M1, M2, M9a | `unicli image resize` + `unicli image crop`  |
| **M9d** | Image: rotate, flip, strip      | M1, M2, M9a | `unicli image rotate/flip/strip`             |
| **M9z** | Image: create (ImageMagick)     | M1, M2, M9a | `unicli image create`                        |
| **M10** | Video module | M1, M2 | `unicli video` commands |
| **M11** | Audio module | M1, M2 | `unicli audio` commands |
| **M12** | PDF module | M1, M2 | `unicli pdf` commands |

### Independence guarantee

- M6 (autocomplete) and M7 (alias) can be built in parallel with M3–M5 (download engines) - they share only M1
- M9–M12 (media modules) are fully parallel with each other after M1 and M2
- M8 (hardening) is the only milestone that intentionally touches all prior work - it is always last before a release
