package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/linuxmatters/jivedrop/internal/cli"
	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
)

// HugoWorkflow implements the Workflow interface for Hugo mode.
// It reads metadata from Hugo frontmatter and supports frontmatter updates after encoding.
type HugoWorkflow struct {
	// hugoMetadata is set during CollectMetadata and read during PostEncode
	hugoMetadata *encoder.EpisodeMetadata
}

// Validate checks Hugo-specific arguments and file existence.
func (h *HugoWorkflow) Validate() error {
	if CLI.EpisodeMD == "" {
		return fmt.Errorf("hugo mode requires episode markdown file as second argument")
	}

	if !strings.HasSuffix(strings.ToLower(CLI.EpisodeMD), ".md") {
		return fmt.Errorf("episode markdown file must have .md extension: %s", CLI.EpisodeMD)
	}

	if _, err := os.Stat(CLI.AudioFile); err != nil {
		return fmt.Errorf("audio file not accessible: %w", err)
	}

	if _, err := os.Stat(CLI.EpisodeMD); err != nil {
		return fmt.Errorf("episode file not accessible: %w", err)
	}

	if CLI.Cover != "" {
		if _, err := os.Stat(CLI.Cover); err != nil {
			return fmt.Errorf("cover art not accessible: %w", err)
		}
	}

	return nil
}

// CollectMetadata parses Hugo frontmatter, applies defaults and flag overrides,
// and resolves the cover art path.
func (h *HugoWorkflow) CollectMetadata() (id3.TagInfo, string, error) {
	metadata, err := encoder.ParseEpisodeMetadata(CLI.EpisodeMD)
	if err != nil {
		return id3.TagInfo{}, "", fmt.Errorf("failed to parse episode metadata: %w", err)
	}
	h.hugoMetadata = metadata

	// Seed with frontmatter values and Hugo defaults; flags override below.
	episodeNum := metadata.Episode
	episodeTitle := metadata.Title
	artist := HugoDefaultArtist
	comment := HugoDefaultComment
	date := encoder.FormatDateForID3(metadata.Date)
	var album string

	// Allow flag overrides
	if CLI.Artist != "" {
		artist = CLI.Artist
	}
	if CLI.Album != "" {
		album = CLI.Album
	} else {
		album = artist // Inherit from artist
	}
	if CLI.Comment != "" {
		comment = CLI.Comment
	}
	if CLI.Title != "" {
		episodeTitle = CLI.Title
	}
	if CLI.Num != "" {
		episodeNum = CLI.Num
	}
	if _, err := encoder.ParseEpisodeNumber(episodeNum); err != nil {
		return id3.TagInfo{}, "", fmt.Errorf("invalid episode number: %w", err)
	}
	if CLI.Date != "" {
		date = CLI.Date
	}

	var coverArtPath string
	if CLI.Cover != "" {
		coverArtPath = CLI.Cover
	} else {
		coverArtPath, err = encoder.ResolveCoverArtPath(CLI.EpisodeMD, metadata.EpisodeImage)
		if err != nil {
			return id3.TagInfo{}, "", fmt.Errorf("failed to resolve cover art: %w", err)
		}
	}

	tagInfo := id3.TagInfo{
		EpisodeNumber: episodeNum,
		Title:         episodeTitle,
		Artist:        artist,
		Album:         album,
		Date:          date,
		Comment:       comment,
	}

	return tagInfo, coverArtPath, nil
}

// PostEncode displays podcast statistics and handles frontmatter comparison and update prompting.
func (h *HugoWorkflow) PostEncode(stats *encoder.FileStats, outputPath string) error {
	printPodcastStats(stats)

	needsUpdate := false
	if h.hugoMetadata.PodcastDuration != "" && h.hugoMetadata.PodcastDuration != stats.DurationString {
		cli.PrintWarning(fmt.Sprintf("Duration mismatch: frontmatter has %s, calculated %s",
			h.hugoMetadata.PodcastDuration, stats.DurationString))
		needsUpdate = true
	}
	if h.hugoMetadata.PodcastBytes > 0 && h.hugoMetadata.PodcastBytes != stats.FileSizeBytes {
		cli.PrintWarning(fmt.Sprintf("File size mismatch: frontmatter has %d, calculated %d",
			h.hugoMetadata.PodcastBytes, stats.FileSizeBytes))
		needsUpdate = true
	}

	// Prompt user to update frontmatter if values differ or are missing
	if needsUpdate {
		promptAndUpdateFrontmatter(CLI.EpisodeMD, "\nUpdate frontmatter with new values? [y/N]: ", stats.DurationString, stats.FileSizeBytes)
	} else if h.hugoMetadata.PodcastDuration == "" || h.hugoMetadata.PodcastBytes == 0 {
		promptAndUpdateFrontmatter(CLI.EpisodeMD, "\nAdd podcast_duration and podcast_bytes to frontmatter? [y/N]: ", stats.DurationString, stats.FileSizeBytes)
	}

	return nil
}

// Ensure HugoWorkflow implements Workflow at compile time.
var _ Workflow = (*HugoWorkflow)(nil)
