# Jivedrop 🪩

> Drop your podcast .wav into a shiny MP3, AAC, or Opus with metadata, cover art, and all

## The Groove

Jivedrop takes your mixed podcast audio (WAV/FLAC) and outputs RSS-ready podcast files with optimised encoding, embedded artwork, and complete metadata. Choose MP3 for universal compatibility, AAC for Apple-recommended quality, or Opus for modern Android and web delivery. One command, distribution-ready output.

### Example Output

<div align="center"><img alt="Jivedrop Demo" src=".github/jivedrop.gif" width="600" /></div>

### What's Cooking

- 🎵 **Multi-format output** via `--format mp3|aac|opus` (default: mp3)
  - 🎸 **MP3** CBR 112kbps mono / 192kbps stereo, 44.1kHz, LAME quality 3, 20.5kHz lowpass
  - 🍏 **AAC** CBR 64kbps mono / 128kbps stereo, 44.1kHz, `.m4a` (Apple-recommended)
  - 🔊 **Opus** VBR ~32kbps mono / ~48kbps stereo, 48kHz, `.opus` (Android/web)
- 🏷️ **Format-native metadata** - correct tags for each container
  - MP3: ID3v2.4 tags with embedded cover art
  - AAC: iTunes MP4 atoms with embedded cover art
  - Opus: Vorbis comments (text tags; no embedded cover)
  - Episode title, number, album, artist, date, comment
  - Podcast enclosure stats for duration and bytes
- ♊ **Dual-mode workflow**
  - 📝 **Hugo mode** read metadata from episode markdown
  - 🎙️ **Standalone mode** specify metadata via flags
- 🚀 **Single binary** Just drop and encode
  - 🐧 **Linux** (amd64 and aarch64)
  - 🍏 **macOS** (x86 and Apple Silicon)

## Usage

### Hugo Mode (Integrated Workflow)

For podcasts using Hugo static site generator and the something like [Castanet](https://github.com/mattstratton/castanet), Jivedrop reads metadata from episode markdown:

**Hugo mode automatically:**
- Reads episode title and number from frontmatter
- Locates cover art from `episode_image` field
- Applies Linux Matters defaults (artist, album, comment)
- Outputs frontmatter-ready values for `podcast_duration` and `podcast_bytes`
- Prompts to update Hugo frontmatter

```bash
# Basic encoding (MP3 by default)
jivedrop LMP67.flac episode/67.md

# Encode as AAC for Apple-recommended distribution
jivedrop LMP67.flac episode/67.md --format aac

# Override Hugo defaults
jivedrop LMP67.flac episode/67.md --artist "Ubuntu Podcast" --comment "https://ubuntupodcast.org"
```
### Standalone Mode (Universal Workflow)

**Standalone mode features:**
- Required flags: `--title`, `--num`, and `--cover`
- Optional metadata: `--artist`, `--album`, `--date`, `--comment`, `--format`
- Smart filename generation: `{artist}-{num}.{ext}` or `episode-{num}.{ext}`
- Album defaults to artist value if not specified

For podcasts without Hugo, specify metadata via flags:

```bash
# Minimal (title, episode number, and cover art required)
jivedrop audio.flac \
  --title "Terminal Full of Sparkles" \
  --num 66 \
  --cover artwork.png

# Encode as Opus (note: Opus is not accepted by Apple Podcasts)
jivedrop audio.flac \
  --title "Terminal Full of Sparkles" \
  --num 66 \
  --cover artwork.png \
  --format opus

# Full metadata
jivedrop audio.flac \
  --title "Terminal Full of Sparkles" \
  --num 66 \
  --artist "Linux Matters" \
  --album "All Seasons" \
  --date "2025-10" \
  --comment "https://linuxmatters.sh/66" \
  --cover artwork.png \
  --format aac
```

## CLI Reference

```
Usage:
  Hugo mode:
    jivedrop <audio-file> <episode-md> [flags]
  Standalone mode:
    jivedrop <audio-file> --title TEXT --num NUMBER --cover PATH [flags]


Arguments:
  [<audio-file>]  Path to audio file (WAV, FLAC)
  [<episode-md>]  Path to episode markdown file (Hugo mode)


Flags:
  -h, --help     Show context-sensitive help.
  --num          Episode number, must be a non-negative integer (required in standalone mode)
  --title        Episode title (required in standalone mode)
  --artist       Artist name (defaults to 'Linux Matters' in Hugo mode)
  --album        Album name (defaults to artist value if omitted)
  --date         Release date (YYYY-MM-DD format)
  --comment      Comment URL (defaults to 'https://linuxmatters.sh' in Hugo mode)
  --cover        Cover art path (required in standalone mode)
  --output-path  Output file or directory path
  --format       Output format: mp3, aac, or opus (default: "mp3")
  --stereo       Encode as stereo at 192kbps (default: mono at 112kbps)
  --version      Show version information
```

### Output
- Hugo mode:        `LMP{num}.{ext}` (or `{artist}-{num}.{ext}` with `--artist` override)
- Standalone mode:  `{artist}-{num}.{ext}` (or `episode-{num}.{ext}` without `--artist`)

Where `{ext}` is `.mp3`, `.m4a`, or `.opus` depending on `--format`.

### Encoding settings

| Format | Mono | Stereo | Sample rate | Notes |
|--------|------|--------|-------------|-------|
| MP3 (default) | 112 kbps CBR | 192 kbps CBR | 44.1 kHz | LAME quality 3, 20.5 kHz lowpass |
| AAC | 64 kbps CBR | 128 kbps CBR | 44.1 kHz | AAC-LC, `.m4a` (ipod muxer), no lowpass |
| Opus | ~32 kbps VBR | ~48 kbps VBR | 48 kHz | libopus, `.opus`, no lowpass; 48 kHz is Opus's native rate |

### Metadata tags

Tags are written natively by the muxer for each format.

**MP3: ID3v2.4**
- `TIT2`: `{num}: {title}`
- `TALB`: `{album}` (omitted if not provided)
- `TRCK`: `{num}`
- `TPE1`: `{artist}` (omitted if not provided)
- `TDRC`: `{date}` (defaults to current YYYY-MM)
- `COMM`: `{comment}` (omitted if not provided)
- `APIC`: Cover art (PNG, front cover)

**AAC: iTunes MP4 atoms**

Same fields as MP3, written as MP4 atoms. Cover art embedded.

**Opus: Vorbis comments**

Same text fields as MP3. Cover art is not embedded in Opus files.

## Build

Jivedrop uses [ffmpeg-statigo](https://github.com/linuxmatters/ffmpeg-statigo) for FFmpeg static bindings.

```bash
# Setup or update ffmpeg-statigo submodule and library
just setup

# Build and test
just build        # Build binary
just test         # Run tests
just test-encoder # Test encoder
```

## Why Jivedrop?

FFmpeg's CLI can absolutely encode podcast-ready audio with metadata. But getting the incantation right for CBR encoding, mono downmix, format-native tags, embedded artwork, and correct lowpass filtering requires a sprawling command line you'll never remember. Switch from MP3 to AAC and every option changes. Add Hugo frontmatter parsing on top and you're writing a script.

Jivedrop wraps the fiddly bits into a single binary that speaks Hugo natively. Drop your WAV, point at your episode markdown, pick your format, and get distribution-ready output with duration and byte counts ready to paste back into your frontmatter.
