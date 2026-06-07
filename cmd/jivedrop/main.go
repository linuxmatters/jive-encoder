package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/kong"
	"github.com/charmbracelet/x/term"
	"github.com/linuxmatters/jivedrop/internal/cli"
	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
	"github.com/linuxmatters/jivedrop/internal/ui"
)

// version is set via ldflags at build time: "dev" for local builds, the git
// tag (e.g. "v0.1.0") for releases.
var version = "dev"

// coverArtResult carries the outcome of concurrent cover art processing back
// to the encode pipeline.
type coverArtResult struct {
	data []byte
	err  error
}

// WorkflowMode selects how metadata is sourced: from Hugo frontmatter or from
// CLI flags alone.
type WorkflowMode int

const (
	// HugoMode reads metadata from an episode markdown file's frontmatter.
	HugoMode WorkflowMode = iota
	// StandaloneMode takes all metadata from CLI flags.
	StandaloneMode
)

var CLI struct {
	AudioFile string `arg:"" name:"audio-file" help:"Path to audio file (WAV, FLAC)" optional:""`
	EpisodeMD string `arg:"" name:"episode-md" help:"Path to episode markdown file (Hugo mode)" optional:""`

	// Metadata flags (standalone mode or Hugo overrides)
	Num        string `help:"Episode number"`
	Title      string `help:"Episode title"`
	Artist     string `help:"Artist name (defaults to 'Linux Matters' in Hugo mode)"`
	Album      string `help:"Album name (defaults to artist value if omitted)"`
	Date       string `help:"Release date (YYYY-MM-DD format)"`
	Comment    string `help:"Comment URL (defaults to 'https://linuxmatters.sh' in Hugo mode)"`
	Cover      string `help:"Cover art path"`
	OutputPath string `help:"Output file or directory path"`

	// Encoding options
	Stereo  bool `help:"Encode as stereo at 192kbps (default: mono at 112kbps)"`
	Version bool `help:"Show version information"`
}

// detectMode determines if this is Hugo or Standalone workflow
func detectMode(audioFile, episodeMD string) WorkflowMode {
	// With no audio file the mode is irrelevant; run() shows help and exits.
	if audioFile == "" {
		return HugoMode
	}

	// A .md second argument signals Hugo mode.
	if episodeMD != "" && strings.HasSuffix(strings.ToLower(episodeMD), ".md") {
		return HugoMode
	}

	return StandaloneMode
}

// sanitiseForFilename lowercases the string, replaces spaces with hyphens, and
// strips anything that is not alphanumeric, hyphen, underscore, or dot, so the
// result is safe to use as a filename.
func sanitiseForFilename(s string) string {
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ToLower(s)
	return strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return -1
	}, s)
}

// generateFilename creates the output filename based on mode and metadata.
// cliArtist is the raw --artist flag value, used in Hugo mode to decide whether
// the default LMP prefix is overridden; artist is the resolved metadata artist.
func generateFilename(mode WorkflowMode, num, artist, cliArtist string) string {
	if mode == HugoMode {
		// Hugo mode: LMP{num}.mp3 unless artist is overridden
		if cliArtist != "" && cliArtist != HugoDefaultArtist {
			sanitisedArtist := sanitiseForFilename(artist)
			return fmt.Sprintf("%s-%s.mp3", sanitisedArtist, num)
		}
		return fmt.Sprintf("%s%s.mp3", HugoDefaultPrefix, num)
	}

	// Standalone mode: {artist}-{num}.mp3 or episode-{num}.mp3 fallback
	if artist != "" {
		sanitisedArtist := sanitiseForFilename(artist)
		return fmt.Sprintf("%s-%s.mp3", sanitisedArtist, num)
	}

	return fmt.Sprintf("episode-%s.mp3", num)
}

// resolveOutputPath determines final output file path. outputPath is the raw
// --output-path flag value; cliArtist is the raw --artist flag value passed
// through to generateFilename.
func resolveOutputPath(mode WorkflowMode, num, artist, cliArtist, outputPath string) (string, error) {
	if outputPath == "" {
		// No path given: write a generated filename in the current directory.
		filename := generateFilename(mode, num, artist, cliArtist)
		return filename, nil
	}

	stat, err := os.Stat(outputPath)
	if err == nil {
		if stat.IsDir() {
			filename := generateFilename(mode, num, artist, cliArtist)
			return filepath.Join(outputPath, filename), nil
		}
		return outputPath, nil
	}

	// A trailing slash on a non-existent path means the user meant a directory,
	// which must already exist; treat the absence as an error.
	if strings.HasSuffix(outputPath, "/") {
		return "", fmt.Errorf("output directory does not exist: %s", outputPath)
	}

	// Treat the path as a file; its parent directory must exist.
	dir := filepath.Dir(outputPath)
	if dir != "." && dir != "" {
		if stat, err := os.Stat(dir); err != nil || !stat.IsDir() {
			return "", fmt.Errorf("output directory does not exist: %s", dir)
		}
	}

	return outputPath, nil
}

// EncodeRequest carries everything the encode pipeline needs, sourced from the
// CLI flags by the caller so encode itself reads no package-level state.
type EncodeRequest struct {
	Mode         WorkflowMode
	TagInfo      id3.TagInfo
	CoverArtPath string
	OutputPath   string
	AudioFile    string
	EpisodeMD    string
	Stereo       bool
}

// printEncodePlan prints the pre-encode summary: the request metadata lines and
// the encoder's resolved input-info line. enc must already be initialised, since
// the input line reads enc.GetInputInfo().
func printEncodePlan(req EncodeRequest, enc *encoder.Encoder) {
	cli.PrintSuccessLabel("Ready to encode:", fmt.Sprintf("%s -> MP3", req.AudioFile))
	cli.PrintLabelValue("• Episode:", fmt.Sprintf("%s - %s", req.TagInfo.EpisodeNumber, req.TagInfo.Title))
	if req.Mode == HugoMode {
		cli.PrintLabelValue("• Episode markdown:", req.EpisodeMD)
	}
	cli.PrintLabelValue("• Output:", req.OutputPath)
	if req.Stereo {
		cli.PrintLabelValue("• Encoding mode:", "Stereo 192kbps")
	} else {
		cli.PrintLabelValue("• Encoding mode:", "Mono 112kbps")
	}

	sampleRate, channels, format := enc.GetInputInfo()
	channelMode := encoder.FormatChannelMode(channels)
	cli.PrintLabelValue("• Input:", fmt.Sprintf("%s %dHz %s", format, sampleRate, channelMode))
}

// encodeOutcome reports how the Bubbletea encoding UI finished. err is non-nil
// when the run failed; partialFile is true when that failure left a truncated
// MP3 that the caller must discard (cancel or encode error, but not a UI error).
type encodeOutcome struct {
	err         error
	partialFile bool
}

// runEncodeUI drives the Bubbletea encoding UI to completion. It detects a TTY,
// builds the matching program, runs it, and reports the resolved outcome. The
// caller owns partial-file cleanup, guided by outcome.partialFile.
func runEncodeUI(enc *encoder.Encoder, outputMode string, outputBitrate int) encodeOutcome {
	// Drive the TUI only on a real terminal. Without a TTY the renderer is
	// disabled so no ANSI box-drawing or cursor escapes reach the pipe.
	isTTY := term.IsTerminal(os.Stdout.Fd())
	encodeModel := ui.NewEncodeModel(enc, outputMode, outputBitrate, !isTTY)
	var p *tea.Program
	if isTTY {
		p = tea.NewProgram(encodeModel, tea.WithFPS(60))
	} else {
		// WithoutRenderer stops output; WithInput(nil) stops Bubbletea
		// opening /dev/tty for input, which would fail in a pipe or CI.
		p = tea.NewProgram(encodeModel, tea.WithoutRenderer(), tea.WithInput(nil))
	}

	finalModel, err := p.Run()
	if err != nil {
		// A UI failure leaves no truncated MP3 to discard.
		return encodeOutcome{err: fmt.Errorf("UI error: %w", err)}
	}

	if encModel, ok := finalModel.(*ui.EncodeModel); ok {
		if encModel.Cancelled() {
			// User interrupted with Ctrl+C. Encode has already returned (the model
			// quits only after EncodingCompleteMsg), so the caller's deferred Close
			// is safe. Report the interrupt; the caller discards the truncated MP3.
			return encodeOutcome{err: fmt.Errorf("encoding cancelled"), partialFile: true}
		}
		if encModel.Error() != nil {
			return encodeOutcome{err: fmt.Errorf("encoding failed: %w", encModel.Error()), partialFile: true}
		}
	}

	// tea.Printf/Println no-op under WithoutRenderer, so emit the encode-stage
	// line directly from here when running without a TTY. This mirrors the TTY
	// completeView (which reports the encode finishing, not the whole job);
	// cover-art and ID3 work still follow, and the final-artefact line marks
	// success.
	if !isTTY {
		fmt.Println("Audio encoded, embedding metadata...")
	}

	return encodeOutcome{}
}

// embedMetadata finishes the job after a successful encode: receive the
// concurrently scaled cover art, write ID3 tags, and extract file statistics.
// Returns nil stats (with nil error) when stats extraction fails but the MP3 was
// written successfully.
func embedMetadata(req EncodeRequest, enc *encoder.Encoder, coverArtChan <-chan coverArtResult) (*encoder.FileStats, error) {
	tagInfo := req.TagInfo

	coverResult := <-coverArtChan
	if coverResult.err != nil {
		cli.PrintInfo(fmt.Sprintf("MP3 file created but missing cover art: %s", req.OutputPath))
		return nil, fmt.Errorf("failed to process cover art: %w", coverResult.err)
	}

	fmt.Println("\nEmbedding ID3v2 tags...")
	tagInfo.CoverArtData = coverResult.data

	if err := id3.WriteTags(req.OutputPath, tagInfo); err != nil {
		cli.PrintInfo(fmt.Sprintf("MP3 file created but missing metadata: %s", req.OutputPath))
		return nil, fmt.Errorf("failed to write ID3 tags: %w", err)
	}

	cli.PrintSuccessLabel("Complete:", req.OutputPath)

	// Extract file statistics using duration from encoder (avoids re-opening file)
	durationSecs := enc.GetDurationSecs()
	stats, err := encoder.GetFileStats(req.OutputPath, durationSecs)
	if err != nil {
		cli.PrintWarning(fmt.Sprintf("Could not extract file statistics: %v", err))
		return nil, nil
	}

	return stats, nil
}

// encode orchestrates the full encoding pipeline: print the plan, create and
// initialise the encoder, scale cover art concurrently, run the Bubbletea UI,
// handle the outcome, then embed metadata and extract statistics. Returns nil
// stats (with nil error) when stats extraction fails but the MP3 was written
// successfully.
func encode(req EncodeRequest) (*encoder.FileStats, error) {
	enc, err := encoder.New(encoder.Config{
		InputPath:  req.AudioFile,
		OutputPath: req.OutputPath,
		Stereo:     req.Stereo,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create encoder: %w", err)
	}
	defer enc.Close()

	if err := enc.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize encoder: %w", err)
	}

	printEncodePlan(req, enc)

	// Process cover art concurrently so scaling overlaps the encode.
	coverArtChan := make(chan coverArtResult, 1)
	go func() {
		if req.CoverArtPath == "" {
			coverArtChan <- coverArtResult{data: nil, err: nil}
			return
		}

		artwork, artErr := id3.ScaleCoverArt(req.CoverArtPath)
		coverArtChan <- coverArtResult{data: artwork, err: artErr}
	}()

	outcome := runEncodeUI(enc, enc.ChannelMode(), enc.Bitrate())
	if outcome.err != nil {
		if outcome.partialFile {
			// Discard the truncated MP3 so a cancelled or failed run leaves no
			// partial file.
			os.Remove(req.OutputPath)
		}
		return nil, outcome.err
	}

	return embedMetadata(req, enc, coverArtChan)
}

func main() {
	os.Exit(run())
}

func run() int {
	ctx := kong.Parse(&CLI,
		kong.Name("jivedrop"),
		kong.Description("Drop the mix, ship the show—metadata, cover art, and all."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
		kong.Help(cli.StyledHelpPrinter(kong.HelpOptions{Compact: true})),
	)

	if CLI.Version {
		cli.PrintVersion(version)
		return 0
	}

	if CLI.AudioFile == "" {
		_ = ctx.PrintUsage(false)
		return 0
	}

	mode := detectMode(CLI.AudioFile, CLI.EpisodeMD)
	opts := CLIOptions{
		EpisodeMD: CLI.EpisodeMD,
		Num:       CLI.Num,
		Title:     CLI.Title,
		Artist:    CLI.Artist,
		Album:     CLI.Album,
		Date:      CLI.Date,
		Comment:   CLI.Comment,
		Cover:     CLI.Cover,
	}
	wf := newWorkflow(mode, opts)

	// Audio-file existence is mode-independent, so check it once here before
	// the mode-specific workflow validation.
	if _, err := os.Stat(CLI.AudioFile); err != nil {
		cli.PrintError(fmt.Errorf("audio file not accessible: %w", err).Error())
		return 1
	}

	if err := wf.Validate(); err != nil {
		cli.PrintError(err.Error())
		return 1
	}

	tagInfo, coverArtPath, err := wf.CollectMetadata()
	if err != nil {
		cli.PrintError(err.Error())
		return 1
	}

	outputPath, err := resolveOutputPath(mode, tagInfo.EpisodeNumber, tagInfo.Artist, CLI.Artist, CLI.OutputPath)
	if err != nil {
		cli.PrintError(fmt.Sprintf("Failed to resolve output path: %v", err))
		return 1
	}

	stats, err := encode(EncodeRequest{
		Mode:         mode,
		TagInfo:      tagInfo,
		CoverArtPath: coverArtPath,
		OutputPath:   outputPath,
		AudioFile:    CLI.AudioFile,
		EpisodeMD:    CLI.EpisodeMD,
		Stereo:       CLI.Stereo,
	})
	if err != nil {
		cli.PrintError(err.Error())
		return 1
	}

	// Stats may be nil when extraction failed but encoding succeeded
	if stats == nil {
		return 0
	}

	if err := wf.PostEncode(stats); err != nil {
		cli.PrintError(err.Error())
		return 1
	}

	return 0
}
