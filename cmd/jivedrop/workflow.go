package main

import (
	"fmt"

	"github.com/linuxmatters/jivedrop/internal/cli"
	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
)

// Workflow defines the mode-specific operations for Hugo and Standalone workflows.
// resolveOutputPath remains a shared package-level helper called from run().
type Workflow interface {
	// Validate checks mode-specific arguments and file existence.
	Validate() error

	// CollectMetadata gathers ID3 tag info and cover art path for the current mode.
	// The cover art path is returned separately because it feeds the concurrent
	// cover art goroutine, not TagInfo directly.
	CollectMetadata() (id3.TagInfo, string, error)

	// PostEncode handles post-encoding operations: stats display and,
	// in Hugo mode, frontmatter comparison and update prompting.
	PostEncode(stats *encoder.FileStats, outputPath string) error
}

// printPodcastStats displays the common podcast statistics shared by every workflow.
func printPodcastStats(stats *encoder.FileStats) {
	fmt.Println("\nPodcast statistics:")
	cli.PrintLabelValue("•   podcast_duration:", stats.DurationString)
	cli.PrintLabelValue("•   podcast_bytes:", fmt.Sprintf("%d", stats.FileSizeBytes))
}

// CLIOptions holds the parsed CLI fields a workflow needs. It is built once in
// run() from the global CLI, confining global reads to the construction site so
// workflow methods read their inputs from receiver data instead.
type CLIOptions struct {
	AudioFile string
	EpisodeMD string
	Num       string
	Title     string
	Artist    string
	Album     string
	Date      string
	Comment   string
	Cover     string
}

// newWorkflow returns the Workflow implementation for the given mode, populated
// with the parsed CLI options.
func newWorkflow(mode WorkflowMode, opts CLIOptions) Workflow {
	switch mode {
	case HugoMode:
		return &HugoWorkflow{opts: opts}
	case StandaloneMode:
		return &StandaloneWorkflow{opts: opts}
	default:
		panic("unknown workflow mode")
	}
}
