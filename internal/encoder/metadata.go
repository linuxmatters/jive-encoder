package encoder

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// EpisodeMetadata holds parsed episode information from Hugo frontmatter
type EpisodeMetadata struct {
	Episode         string    `yaml:"episode"`
	Title           string    `yaml:"title"`
	Date            time.Time `yaml:"Date"`
	EpisodeImage    string    `yaml:"episode_image"`
	PodcastDuration string    `yaml:"podcast_duration"`
	PodcastBytes    int64     `yaml:"podcast_bytes"`
}

// ParseEpisodeMetadata extracts metadata from a Hugo markdown file
func ParseEpisodeMetadata(markdownPath string) (*EpisodeMetadata, error) {
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read episode file: %w", err)
	}

	frontmatter, err := extractFrontmatter(string(content))
	if err != nil {
		return nil, err
	}

	var meta EpisodeMetadata
	if err := yaml.Unmarshal([]byte(frontmatter), &meta); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	if meta.Episode == "" {
		return nil, fmt.Errorf("missing required field: episode")
	}
	if meta.Title == "" {
		return nil, fmt.Errorf("missing required field: title")
	}
	if meta.EpisodeImage == "" {
		return nil, fmt.Errorf("missing required field: episode_image")
	}

	return &meta, nil
}

// extractFrontmatter extracts YAML content between --- delimiters
func extractFrontmatter(content string) (string, error) {
	lines := strings.Split(content, "\n")

	start, end, err := findFrontmatterBounds(lines)
	if err != nil {
		return "", err
	}

	return strings.Join(lines[start:end], "\n"), nil
}

// findFrontmatterBounds locates the start and end indices of frontmatter content.
// Returns the line index after the opening --- and the line index of the closing ---.
func findFrontmatterBounds(lines []string) (start, end int, err error) {
	delimiterCount := 0

	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			delimiterCount++
			switch delimiterCount {
			case 1:
				start = i + 1
			case 2:
				end = i
				return start, end, nil
			}
		}
	}

	return 0, 0, fmt.Errorf("invalid frontmatter: expected two '---' delimiters, found %d", delimiterCount)
}

// ResolveCoverArtPath resolves the episode_image path to an absolute path
// The episode_image in frontmatter is relative to the markdown file
func ResolveCoverArtPath(markdownPath, episodeImage string) (string, error) {
	markdownDir := filepath.Dir(markdownPath)

	// A "./" prefix means the image sits beside the markdown file.
	if after, ok := strings.CutPrefix(episodeImage, "./"); ok {
		coverPath := filepath.Join(markdownDir, after)
		coverPath, err := filepath.Abs(coverPath)
		if err != nil {
			return "", fmt.Errorf("failed to resolve cover art path: %w", err)
		}

		if _, err := os.Stat(coverPath); err != nil {
			return "", fmt.Errorf("cover art not found: %s", coverPath)
		}

		return coverPath, nil
	}

	// Otherwise the path is rooted at the Hugo site, served from static/.
	projectRoot, err := findProjectRoot(markdownDir)
	if err != nil {
		return "", err
	}

	coverPath := filepath.Join(projectRoot, "static", strings.TrimPrefix(episodeImage, "/"))
	coverPath, err = filepath.Abs(coverPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve cover art path: %w", err)
	}

	if _, err := os.Stat(coverPath); err != nil {
		return "", fmt.Errorf("cover art not found: %s", coverPath)
	}

	return coverPath, nil
}

// findProjectRoot walks up the directory tree to find the Hugo project root
// (directory containing "static" folder)
func findProjectRoot(startPath string) (string, error) {
	currentPath := startPath

	for {
		staticPath := filepath.Join(currentPath, "static")
		if info, err := os.Stat(staticPath); err == nil && info.IsDir() {
			return currentPath, nil
		}

		parentPath := filepath.Dir(currentPath)

		// filepath.Dir returns its input at the filesystem root: stop there.
		if parentPath == currentPath {
			return "", fmt.Errorf("could not find Hugo project root (no 'static' directory found)")
		}

		currentPath = parentPath
	}
}

// FormatDateForID3 formats a time.Time to "YYYY-MM" format for ID3 TDRC tag
func FormatDateForID3(t time.Time) string {
	return t.Format("2006-01")
}

// UpdateFrontmatter updates podcast_duration and podcast_bytes in the markdown file
func UpdateFrontmatter(markdownPath, duration string, bytes int64) error {
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	start, end, err := findFrontmatterBounds(lines)
	if err != nil {
		return fmt.Errorf("invalid frontmatter format: %w", err)
	}

	// Rewrite existing keys in place, tracking which were present.
	updated := false
	bytesUpdated := false

	for i := start; i < end; i++ {
		line := lines[i]

		if strings.HasPrefix(strings.TrimSpace(line), "podcast_duration:") {
			lines[i] = fmt.Sprintf("podcast_duration: %s", duration)
			updated = true
		}

		if strings.HasPrefix(strings.TrimSpace(line), "podcast_bytes:") {
			lines[i] = fmt.Sprintf("podcast_bytes: %d", bytes)
			bytesUpdated = true
		}
	}

	// Insert any missing keys just before the closing delimiter.
	if !updated || !bytesUpdated {
		var insertLines []string
		if !updated {
			insertLines = append(insertLines, fmt.Sprintf("podcast_duration: %s", duration))
		}
		if !bytesUpdated {
			insertLines = append(insertLines, fmt.Sprintf("podcast_bytes: %d", bytes))
		}

		lines = slices.Insert(lines, end, insertLines...)
	}

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(markdownPath, []byte(output), 0o644); err != nil { //nolint:gosec // markdownPath is user-provided input path, not tainted
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
