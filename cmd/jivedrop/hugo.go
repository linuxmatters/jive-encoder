package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/linuxmatters/jivedrop/internal/cli"
	"github.com/linuxmatters/jivedrop/internal/encoder"
	"github.com/linuxmatters/jivedrop/internal/id3"
)

// Hugo mode metadata defaults for the Linux Matters podcast.
const (
	HugoDefaultArtist  = "Linux Matters"
	HugoDefaultComment = "https://linuxmatters.sh"
	HugoDefaultPrefix  = "LMP"
)

// HugoWorkflow implements the Workflow interface for Hugo mode.
// It reads metadata from Hugo frontmatter and supports frontmatter updates after encoding.
type HugoWorkflow struct {
	// opts carries the parsed CLI fields, populated at construction.
	opts CLIOptions
	// hugoMetadata is set during CollectMetadata and read during PostEncode
	hugoMetadata *encoder.EpisodeMetadata
}

// Validate checks Hugo-specific arguments and file existence.
func (h *HugoWorkflow) Validate() error {
	if h.opts.EpisodeMD == "" {
		return fmt.Errorf("hugo mode requires episode markdown file as second argument")
	}

	if !strings.HasSuffix(strings.ToLower(h.opts.EpisodeMD), ".md") {
		return fmt.Errorf("episode markdown file must have .md extension: %s", h.opts.EpisodeMD)
	}

	if _, err := os.Stat(h.opts.EpisodeMD); err != nil {
		return fmt.Errorf("episode file not accessible: %w", err)
	}

	if h.opts.Cover != "" {
		if _, err := os.Stat(h.opts.Cover); err != nil {
			return fmt.Errorf("cover art not accessible: %w", err)
		}
	}

	return nil
}

// CollectMetadata parses Hugo frontmatter, applies defaults and flag overrides,
// and resolves the cover art path.
func (h *HugoWorkflow) CollectMetadata() (id3.TagInfo, string, error) {
	metadata, err := encoder.ParseEpisodeMetadata(h.opts.EpisodeMD)
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

	if h.opts.Artist != "" {
		artist = h.opts.Artist
	}
	album := resolveAlbum(h.opts.Album, artist)
	if h.opts.Comment != "" {
		comment = h.opts.Comment
	}
	if h.opts.Title != "" {
		episodeTitle = h.opts.Title
	}
	if h.opts.Num != "" {
		episodeNum = h.opts.Num
	}
	if _, err := encoder.ParseEpisodeNumber(episodeNum); err != nil {
		return id3.TagInfo{}, "", fmt.Errorf("invalid episode number: %w", err)
	}
	if h.opts.Date != "" {
		date = h.opts.Date
	}

	var coverArtPath string
	if h.opts.Cover != "" {
		coverArtPath = h.opts.Cover
	} else {
		coverArtPath, err = encoder.ResolveCoverArtPath(h.opts.EpisodeMD, metadata.EpisodeImage)
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
// stats describe the single encoded file regardless of format (duration seconds + byte size),
// so write-back applies unchanged for mp3, opus, or aac. The prompt-on-change guard shows the
// new podcast_duration/podcast_bytes and waits for confirmation before writing, so a non-mp3
// encode cannot silently overwrite values for a different enclosure.
func (h *HugoWorkflow) PostEncode(stats *encoder.FileStats) error {
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
		promptAndUpdateFrontmatter(h.opts.EpisodeMD, "\nUpdate frontmatter with new values? [y/N]: ", stats.DurationString, stats.FileSizeBytes)
	} else if h.hugoMetadata.PodcastDuration == "" || h.hugoMetadata.PodcastBytes == 0 {
		promptAndUpdateFrontmatter(h.opts.EpisodeMD, "\nAdd podcast_duration and podcast_bytes to frontmatter? [y/N]: ", stats.DurationString, stats.FileSizeBytes)
	}

	return nil
}

// promptAndUpdateFrontmatter prompts the user and updates the frontmatter with podcast stats
func promptAndUpdateFrontmatter(markdownPath, promptMsg, duration string, bytes int64) {
	fmt.Print(promptMsg)
	var response string
	_, _ = fmt.Scanln(&response)

	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		if err := encoder.UpdateFrontmatter(markdownPath, duration, bytes); err != nil {
			cli.PrintError(fmt.Sprintf("Failed to update frontmatter: %v", err))
		} else {
			cli.PrintSuccess("Frontmatter updated successfully")
		}
	} else {
		cli.PrintInfo("Frontmatter not updated")
	}
}

// Ensure HugoWorkflow implements Workflow at compile time.
var _ Workflow = (*HugoWorkflow)(nil)
