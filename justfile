# Jivedrop - Just Commands

# List commands
default:
    @just --list

# Check ffmpeg-statigo submodule is present
_check-submodule:
    #!/usr/bin/env bash
    if [ ! -f "third_party/ffmpeg-statigo/go.mod" ]; then
        echo "Error: ffmpeg-statigo submodule not initialised. Run 'just setup' first."
        exit 1
    fi
    if [ ! -f "third_party/ffmpeg-statigo/lib/$(go env GOOS)_$(go env GOARCH)/libffmpeg.a" ]; then
        echo "Error: ffmpeg-statigo library not downloaded. Run 'just setup' first."
        exit 1
    fi

# Get latest stable ffmpeg-statigo release tag from GitHub
_get-latest-tag:
    #!/usr/bin/env bash
    if command -v jq &> /dev/null; then
        curl -s https://api.github.com/repos/linuxmatters/ffmpeg-statigo/releases | \
            jq -r '[.[] | select(.prerelease == false and .draft == false and (.tag_name | startswith("v")))][0].tag_name'
    else
        curl -s https://api.github.com/repos/linuxmatters/ffmpeg-statigo/releases | \
            grep -B5 '"prerelease": false' | grep '"tag_name"' | grep -v 'lib-' | head -1 | cut -d'"' -f4
    fi

# Setup or update ffmpeg-statigo submodule and library
setup:
    #!/usr/bin/env bash
    set -e
    echo "Configuring git for submodule-friendly pulls..."
    git config pull.ff only
    git config submodule.recurse true

    # Get latest stable release tag
    TAG=$(just _get-latest-tag)
    if [ -z "$TAG" ] || [ "$TAG" = "null" ]; then
        echo "Error: Could not fetch latest release tag"
        exit 1
    fi

    # Initialise submodule if not already present
    if [ ! -f "third_party/ffmpeg-statigo/go.mod" ]; then
        echo "Initialising ffmpeg-statigo submodule..."
        git submodule update --init --recursive
    fi

    # Check current version
    cd third_party/ffmpeg-statigo
    git fetch --tags
    CURRENT=$(git describe --tags --exact-match 2>/dev/null || echo "")

    if [ "$CURRENT" = "$TAG" ]; then
        echo "ffmpeg-statigo already at latest version ($TAG)"
        cd ../..
    else
        if [ -n "$CURRENT" ]; then
            echo "Updating ffmpeg-statigo from $CURRENT to $TAG..."
        else
            echo "Setting up ffmpeg-statigo $TAG..."
        fi
        git checkout "$TAG"
        cd ../..

        # Remove old library to force re-download
        rm -f third_party/ffmpeg-statigo/lib/*/libffmpeg.a

        # Stage the submodule change
        git add third_party/ffmpeg-statigo
    fi

    # Download libraries (will skip if correct version already exists)
    echo "Checking ffmpeg-statigo libraries..."
    cd third_party/ffmpeg-statigo && go run ./cmd/download-lib
    cd ../..

    # Check if there are staged changes to commit
    if git diff --cached --quiet third_party/ffmpeg-statigo; then
        echo "Setup complete!"
    else
        echo ""
        echo "Setup complete! Submodule updated to $TAG"
        echo "Don't forget to commit: git commit -m 'chore: update ffmpeg-statigo to $TAG'"
    fi

# Build jivedrop
build: _check-submodule
    #!/usr/bin/env bash
    VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
    echo "Building jivedrop version: $VERSION"
    CGO_ENABLED=1 go build -ldflags="-X main.version=$VERSION" -o jivedrop ./cmd/jivedrop

# Clean build artifacts
clean:
    rm -fv jivedrop 2>/dev/null || true
    @rm testdata/*.mp3 2>/dev/null || true
    @rm testdata/*.m4a 2>/dev/null || true
    @rm testdata/*.opus 2>/dev/null || true

# Install jivedrop to ~/.local/bin
install: build
    @mkdir -p ~/.local/bin 2>/dev/null || true
    @mv ./jivedrop ~/.local/bin/jivedrop
    @echo "Installed jivedrop to ~/.local/bin/jivedrop"
    @echo "Make sure ~/.local/bin is in your PATH"

# Record gif
vhs: build
    @vhs ./jivedrop.tape
    rm LMP67.mp3 2>/dev/null || true

# Test encoder: encode all three formats and assert codec, sample rate, tags, and cover
test-encoder: build
    #!/usr/bin/env bash
    set -euo pipefail

    flac="testdata/LMP67.flac"
    meta="testdata/67.md"
    out="testdata"

    # Per-format expectations: codec, sample rate, expected attached_pic count.
    declare -A ext=(   [mp3]="mp3"   [aac]="m4a"  [opus]="opus" )
    declare -A codec=( [mp3]="mp3"   [aac]="aac"  [opus]="opus" )
    declare -A rate=(  [mp3]="44100" [aac]="44100" [opus]="48000" )
    declare -A cover=( [mp3]="1"     [aac]="1"    [opus]="0" )

    # tag_value KEY FILE — read a tag from format-level first, then stream-level
    # (Opus stores Vorbis comments on the audio stream, not the container).
    tag_value() {
        local key="$1" file="$2" val
        val=$(ffprobe -v error -show_entries "format_tags=$key" -of "default=noprint_wrappers=1:nokey=1" "$file")
        if [ -z "$val" ]; then
            val=$(ffprobe -v error -select_streams a:0 -show_entries "stream_tags=$key" -of "default=noprint_wrappers=1:nokey=1" "$file")
        fi
        printf '%s' "$val"
    }

    fail() { echo "FAIL: $1" >&2; exit 1; }

    for fmt in mp3 aac opus; do
        file="$out/LMP67.${ext[$fmt]}"
        rm -f "$file"

        # Decline the frontmatter-update prompt. Gate on the encode exit status,
        # not the SIGPIPE that "echo n" may receive once jivedrop stops reading.
        set +o pipefail
        echo n | ./jivedrop "$flac" "$meta" --format "$fmt" --output-path "$out/" >/dev/null
        rc=${PIPESTATUS[1]}
        set -o pipefail
        [ "$rc" -eq 0 ] || fail "$fmt: jivedrop exited $rc"
        [ -f "$file" ] || fail "$fmt: expected output $file not created"

        got_codec=$(ffprobe -v error -select_streams a:0 -show_entries stream=codec_name -of "default=noprint_wrappers=1:nokey=1" "$file")
        [ "$got_codec" = "${codec[$fmt]}" ] || fail "$fmt: codec $got_codec, expected ${codec[$fmt]}"

        got_rate=$(ffprobe -v error -select_streams a:0 -show_entries stream=sample_rate -of "default=noprint_wrappers=1:nokey=1" "$file")
        [ "$got_rate" = "${rate[$fmt]}" ] || fail "$fmt: sample rate $got_rate, expected ${rate[$fmt]}"

        title=$(tag_value title "$file")
        case "$title" in
            67:*) ;;
            *) fail "$fmt: title '$title' does not start with episode number '67:'" ;;
        esac
        for key in album artist track date comment; do
            val=$(tag_value "$key" "$file")
            [ -n "$val" ] || fail "$fmt: tag '$key' is missing or empty"
        done

        pics=$(ffprobe -v error -show_entries stream_disposition=attached_pic -of "default=noprint_wrappers=1:nokey=1" "$file" | grep -c '^1$' || true)
        [ "$pics" = "${cover[$fmt]}" ] || fail "$fmt: attached-picture count $pics, expected ${cover[$fmt]}"

        echo "PASS $fmt: codec=$got_codec rate=${got_rate}㎐ tags=ok cover=$pics"
    done

    echo "All formats passed."

# Run linters
lint:
    @go vet ./...
    @gocyclo -top 20 -avg -ignore '_test\.go$' .
    @ineffassign ./...
    @golangci-lint run
    @actionlint

# Run tests
test:
    go test ./...
