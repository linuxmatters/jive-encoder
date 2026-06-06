package main

import (
	"fmt"
	"os"

	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
)

// StandaloneWorkflow implements the Workflow interface for standalone mode.
// Metadata comes entirely from CLI flags.
type StandaloneWorkflow struct {
	// opts carries the parsed CLI fields, populated at construction.
	opts CLIOptions
}

// Validate checks standalone-specific arguments and file existence.
func (s *StandaloneWorkflow) Validate() error {
	if s.opts.Title == "" {
		return fmt.Errorf("standalone mode requires --title flag")
	}

	if s.opts.Num == "" {
		return fmt.Errorf("standalone mode requires --num flag (episode number)")
	}

	if _, err := encoder.ParseEpisodeNumber(s.opts.Num); err != nil {
		return fmt.Errorf("invalid --num flag: %w", err)
	}

	if s.opts.Cover == "" {
		return fmt.Errorf("standalone mode requires --cover flag (cover art path)")
	}

	if _, err := os.Stat(s.opts.Cover); err != nil {
		return fmt.Errorf("cover art not accessible: %w", err)
	}

	return nil
}

// CollectMetadata builds TagInfo from CLI flags.
func (s *StandaloneWorkflow) CollectMetadata() (id3.TagInfo, string, error) {
	album := s.opts.Album
	if album == "" && s.opts.Artist != "" {
		album = s.opts.Artist // Inherit from artist
	}

	tagInfo := id3.TagInfo{
		EpisodeNumber: s.opts.Num,
		Title:         s.opts.Title,
		Artist:        s.opts.Artist,
		Album:         album,
		Date:          s.opts.Date,
		Comment:       s.opts.Comment,
	}

	return tagInfo, s.opts.Cover, nil
}

// PostEncode displays podcast statistics. Standalone mode has no frontmatter to update.
func (s *StandaloneWorkflow) PostEncode(stats *encoder.FileStats, outputPath string) error {
	printPodcastStats(stats)
	return nil
}

// Ensure StandaloneWorkflow implements Workflow at compile time.
var _ Workflow = (*StandaloneWorkflow)(nil)
