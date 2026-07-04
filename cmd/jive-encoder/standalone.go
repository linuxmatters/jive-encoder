package main

import (
	"fmt"
	"os"

	"github.com/linuxmatters/jive-encoder/internal/encoder"
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

// CollectMetadata builds the episode tag metadata from CLI flags.
func (s *StandaloneWorkflow) CollectMetadata() (encoder.Metadata, string, error) {
	album := resolveAlbum(s.opts.Album, s.opts.Artist)

	tags := encoder.Metadata{
		EpisodeNumber: s.opts.Num,
		Title:         s.opts.Title,
		Artist:        s.opts.Artist,
		Album:         album,
		Date:          s.opts.Date,
		Comment:       s.opts.Comment,
	}

	return tags, s.opts.Cover, nil
}

// PostEncode displays podcast statistics. Standalone mode has no frontmatter to update.
func (s *StandaloneWorkflow) PostEncode(stats *encoder.FileStats) error {
	printPodcastStats(stats)
	return nil
}

// Ensure StandaloneWorkflow implements Workflow at compile time.
var _ Workflow = (*StandaloneWorkflow)(nil)
