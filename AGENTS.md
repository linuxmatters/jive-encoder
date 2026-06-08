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
  encoder/               # FFmpeg-based MP3/AAC/Opus encoding via ffmpeg-statigo
    encoder.go           # Core encode pipeline: decode → filter → encode → muxer-native tag
    preset.go            # Per-format preset table (codec, bitrate, sample fmt/rate, muxer, extension, lowpass, cover)
    metadata.go          # Hugo frontmatter parsing (YAML between --- delimiters) + muxer tag assembly
    stats.go             # Duration/filesize extraction from the encoded file
  id3/                   # Cover-art scaling and tag-field carrier (no ID3 writer; FFmpeg muxers write tags)
    artwork.go           # Cover art scaling (1400-3000px range for Apple Podcasts)
    taginfo.go           # TagInfo carrier for episode metadata fields
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

- **Hugo mode**: `jivedrop audio.flac episode.md`: reads metadata from Hugo frontmatter
- **Standalone mode**: `jivedrop audio.flac --title X --num N --cover Y`: explicit flags
- Mode detection: second argument ending in `.md` triggers Hugo mode
- `--format mp3|opus|aac` selects one format per invocation (single value, default `mp3`); Kong rejects unknown values at parse time. Each invocation emits one file with the preset extension

### Hugo Frontmatter

- Required fields in episode markdown: `episode`, `title`, `episode_image`
- `episode` must be a non-empty, non-negative integer (validated by `encoder.ParseEpisodeNumber`); same rule applies to the standalone `--num` flag
- After encoding, Jivedrop calculates `podcast_duration` and `podcast_bytes`
- Write-back is format-agnostic: the stats reflect the single encoded file, whatever format was chosen
- Prompts user to update frontmatter if values differ or are missing

### Encoding Settings

`internal/encoder/preset.go` holds the per-format preset table (the single source of truth for codec, bitrate, sample format, sample rate, muxer, extension, lowpass, cover capability). Mono is the default; `--stereo` selects the stereo bitrate.

- **MP3 (default)**: CBR 112/192kbps, 44.1kHz, sample fmt `s16p`, LAME quality 3, 20.5kHz lowpass; `mp3` muxer → `.mp3`
- **AAC-LC (`--format aac`)**: CBR 64/128kbps, 44.1kHz, sample fmt `fltp`, no lowpass; `ipod` muxer → `.m4a`
- **Opus (`--format opus`)**: VBR ~32/~48kbps, 48kHz (libopus rejects 44.1kHz), sample fmt `flt` (libopus rejects `fltp`), `vbr=on`, compression_level 10, no lowpass; `opus` muxer → `.opus`

### FFmpeg Integration

- Uses `ffmpeg-statigo` submodule for static FFmpeg bindings (no system FFmpeg needed)
- Per-format filter graph: resample to the preset's sample rate and sample format → channel downmix (mono default, stereo keeps channels) → encode. Lowpass is MP3-only
- Per-encoder frame size: `openOutput` runs before `initFilter`, so `initFilter` calls `AVBuffersinkSetFrameSize(sink, encCtx.FrameSize())` to feed each encoder its required frame size (MP3 1152, AAC 1024) unless the encoder advertises `AV_CODEC_CAP_VARIABLE_FRAME_SIZE`
- All FFmpeg types prefixed with `ffmpeg.AV*`

### Metadata

- Tagging is FFmpeg muxer-native: standard keys (`title`/`artist`/`album`/`date`/`comment`/`track`) go into an `AVDictionary` on the output format context before `AVFormatWriteHeader`, so each muxer writes its own format: ID3v2.4 (MP3, via the `id3v2_version=4` WriteHeader muxer option), iTunes MP4 atoms (M4A), Vorbis comments (Opus)
- Title renders `"{episode}: {title}"`; track maps to the episode number; empty fields are skipped
- Cover is an attached-picture stream (`AVDispositionAttachedPic`) written right after the header, for cover-capable formats (MP3, AAC) only. **Opus has no embedded cover** (text tags only)
- `bogem/id3v2` is removed. `internal/id3/` holds only `artwork.go` (cover scaling) and `taginfo.go` (the `TagInfo` carrier)

## Code Conventions

- **British English spelling** in user-facing text and comments
- **Charm v2 libraries** publish under the `charm.land` vanity path, not `github.com/charmbracelet/.../v2`: import `charm.land/bubbletea/v2`, `charm.land/bubbles/v2`, `charm.land/lipgloss/v2`
- **Lipgloss styles** in `internal/cli/styles.go` use the colour palette defined in `internal/cli/colours.go`
- **Kong** for CLI parsing with custom help printer
- **Bubbletea** for interactive progress UI during encoding
- Use `cli.PrintError()` and `cli.PrintInfo()` for user-facing messages
- Wrap errors with context: `fmt.Errorf("failed to X: %w", err)`
- Clean up partial files on encoding failure

## Testing Instructions

- `testdata/` contains sample FLAC, markdown, and artwork files
- Tests output to `testdata/` (`.mp3`/`.m4a`/`.opus`, cleaned by `just clean`)
- Run `just test-encoder` for integration testing with real audio files

## Environment

- **OS:** NixOS with `flake.nix` development shell
- **Shell:** Fish (interactive), bash (scripts)
- **CGO required:** Set `CGO_ENABLED=1` for builds

## FFmpeg-Statigo Submodule

This project uses [`ffmpeg-statigo`](https://github.com/linuxmatters/ffmpeg-statigo) for FFmpeg 8.1 static bindings:

- **Location:** `third_party/ffmpeg-statigo/`
- **Auto-generated files:** `*.gen.go` files (e.g., `functions.gen.go`, `structs.gen.go`), do not edit
- **C headers:** `include/` directory for CGO compilation
- **Libraries:** `lib/<os>_<arch>/libffmpeg.a` (gitignored, downloaded by `just setup`)

For submodule-specific instructions, see `third_party/ffmpeg-statigo/.github/copilot-instructions.md`.
