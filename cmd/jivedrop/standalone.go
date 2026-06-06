package main

import (
	"fmt"
	"os"

	"github.com/linuxmatters/jivedrop/internal/cli"
	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
)

// StandaloneWorkflow implements the Workflow interface for standalone mode.
// Metadata comes entirely from CLI flags.
type StandaloneWorkflow struct{}

// Validate checks standalone-specific arguments and file existence.
func (s *StandaloneWorkflow) Validate() error {
	if CLI.Title == "" {
		return fmt.Errorf("standalone mode requires --title flag")
	}

	if CLI.Num == "" {
		return fmt.Errorf("standalone mode requires --num flag (episode number)")
	}

	if _, err := encoder.ParseEpisodeNumber(CLI.Num); err != nil {
		return fmt.Errorf("invalid --num flag: %w", err)
	}

	if CLI.Cover == "" {
		return fmt.Errorf("standalone mode requires --cover flag (cover art path)")
	}

	if _, err := os.Stat(CLI.AudioFile); err != nil {
		return fmt.Errorf("audio file not accessible: %w", err)
	}

	if _, err := os.Stat(CLI.Cover); err != nil {
		return fmt.Errorf("cover art not accessible: %w", err)
	}

	return nil
}

// CollectMetadata builds TagInfo from CLI flags.
func (s *StandaloneWorkflow) CollectMetadata() (id3.TagInfo, string, error) {
	album := CLI.Album
	if album == "" && CLI.Artist != "" {
		album = CLI.Artist // Inherit from artist
	}

	tagInfo := id3.TagInfo{
		EpisodeNumber: CLI.Num,
		Title:         CLI.Title,
		Artist:        CLI.Artist,
		Album:         album,
		Date:          CLI.Date,
		Comment:       CLI.Comment,
	}

	return tagInfo, CLI.Cover, nil
}

// PostEncode displays podcast statistics. Standalone mode has no frontmatter to update.
func (s *StandaloneWorkflow) PostEncode(stats *encoder.FileStats, outputPath string) error {
	fmt.Println("\nPodcast statistics:")
	cli.PrintLabelValue("•   podcast_duration:", stats.DurationString)
	cli.PrintLabelValue("•   podcast_bytes:", fmt.Sprintf("%d", stats.FileSizeBytes))
	return nil
}

// Ensure StandaloneWorkflow implements Workflow at compile time.
var _ Workflow = (*StandaloneWorkflow)(nil)
