# AGENTS.md

## Setup Commands

**Required first step:**
```bash
just setup
```
Initialises the `ffmpeg-statigo` submodule and downloads platform-specific FFmpeg static libraries.

**Development environment:**
```bash
nix develop  # Enter NixOS development shell (ffmpeg, lame, mediainfo, just, go)
```

## Build and Test Commands

```bash
just build      # Build binary with version from git tags (CGO_ENABLED=1)
just test       # Run all Go tests
just test-encoder  # Integration test: encode testdata/LMP67.flac
just clean      # Remove build artifacts and test outputs (*.mp3)
```

## Architecture

```
cmd/jivedrop/
  main.go                # CLI entry, mode detection (Hugo vs Standalone), argument validation
  workflow.go            # Workflow interface + CLIOptions struct passed to each workflow
  hugo.go                # Hugo-mode workflow (frontmatter-driven)
  standalone.go          # Standalone-mode workflow (flag-driven)
internal/
  encoder/               # FFmpeg-based MP3 encoding via ffmpeg-statigo
    encoder.go           # Core encode pipeline: decode → filter → encode
    metadata.go          # Hugo frontmatter parsing (YAML between --- delimiters)
    stats.go             # Duration/filesize extraction from encoded MP3
  id3/                   # ID3v2.4 tag writing via bogem/id3v2
    writer.go            # Tag frames: TIT2, TALB, TRCK, TPE1, TDRC, COMM, APIC
    artwork.go           # Cover art scaling (1400-3000px range for Apple Podcasts)
  ui/                    # Bubbletea TUI for encoding progress
    encode.go            # Progress model with realtime speed calculation
  cli/                   # Lipgloss-styled output
    help.go              # Custom Kong help printer
    colours.go           # Colour palette (matches Jivefire sibling project)
    styles.go            # Lipgloss styles + Print* helpers (PrintError, PrintInfo, PrintWarning, ...)
third_party/ffmpeg-statigo/  # Git submodule: FFmpeg 8.1 static bindings
```

## Key Patterns

### Dual-Mode CLI

- **Hugo mode**: `jivedrop audio.flac episode.md` — reads metadata from Hugo frontmatter
- **Standalone mode**: `jivedrop audio.flac --title X --num N --cover Y` — explicit flags
- Mode detection: second argument ending in `.md` triggers Hugo mode

### Hugo Frontmatter

- Required fields in episode markdown: `episode`, `title`, `episode_image`
- `episode` must be a non-empty, non-negative integer (validated by `encoder.ParseEpisodeNumber`); same rule applies to the standalone `--num` flag
- After encoding, Jivedrop calculates `podcast_duration` and `podcast_bytes`
- Prompts user to update frontmatter if values differ or are missing

### Encoding Settings

- **Mono (default)**: CBR 112kbps, 44.1kHz, LAME quality 3, 20.5kHz lowpass
- **Stereo (`--stereo`)**: CBR 192kbps, 44.1kHz, LAME quality 3, 20.5kHz lowpass

### FFmpeg Integration

- Uses `ffmpeg-statigo` submodule for static FFmpeg bindings (no system FFmpeg needed)
- Filter graph: resample → channel downmix → LAME encode
- All FFmpeg types prefixed with `ffmpeg.AV*`

## Code Conventions

- **British English spelling** in user-facing text and comments
- **Charm v2 libraries** publish under the `charm.land` vanity path, not `github.com/charmbracelet/.../v2`: import `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`
- **Lipgloss styles** in `internal/cli/styles.go` use the colour palette defined in `internal/cli/colours.go`
- **Kong** for CLI parsing with custom help printer
- **Bubbletea** for interactive progress UI during encoding
- Use `cli.PrintError()` and `cli.PrintInfo()` for user-facing messages
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Clean up partial files on encoding failure

## Error Handling

- Use `cli.PrintError()` and `cli.PrintInfo()` for user-facing messages
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Clean up partial files on encoding failure

## Testing Instructions

- `testdata/` contains sample FLAC, markdown, and artwork files
- Tests output to `testdata/*.mp3` (cleaned by `just clean`)
- Run `just test-encoder` for integration testing with real audio files

## Environment

- **OS:** NixOS with `flake.nix` development shell
- **Shell:** Fish (interactive), bash (scripts)
- **CGO required:** Set `CGO_ENABLED=1` for builds

## FFmpeg-Statigo Submodule

This project uses [`ffmpeg-statigo`](https://github.com/linuxmatters/ffmpeg-statigo) for FFmpeg 8.1 static bindings:

- **Location:** `third_party/ffmpeg-statigo/`
- **Auto-generated files:** `*.gen.go` files (e.g., `functions.gen.go`, `structs.gen.go`) — do not edit
- **C headers:** `include/` directory for CGO compilation
- **Libraries:** `lib/<os>_<arch>/libffmpeg.a` (gitignored, downloaded by `just setup`)

For submodule-specific instructions, see `third_party/ffmpeg-statigo/.github/copilot-instructions.md`.
