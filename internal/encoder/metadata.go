package encoder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ParseEpisodeNumber validates a raw episode number and returns it unchanged
// when valid. An episode number must be non-empty and a non-negative integer,
// so it produces a well-formed muxer track tag and filename. Non-numeric input
// such as "foo" or "67a" is rejected at the boundary.
func ParseEpisodeNumber(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("episode number is required")
	}

	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return "", fmt.Errorf("invalid episode number %q: must be a non-negative integer", s)
	}

	return s, nil
}

// muxerTag pairs a standard muxer metadata key with its value. Ordered pairs
// keep tag emission deterministic across the title/artist/album/date/comment/track set.
type muxerTag struct {
	Key   string
	Value string
}

// buildMuxerTags renders the muxer metadata key/value set from the episode
// fields, skipping empty values. The title uses the "{EpisodeNumber}: {Title}"
// format. The track key carries the episode number.
func buildMuxerTags(m Metadata) []muxerTag {
	var tags []muxerTag

	add := func(key, value string) {
		if value != "" {
			tags = append(tags, muxerTag{Key: key, Value: value})
		}
	}

	if m.Title != "" {
		add("title", fmt.Sprintf("%s: %s", m.EpisodeNumber, m.Title))
	}
	add("artist", m.Artist)
	add("album", m.Album)
	add("date", m.Date)
	add("comment", m.Comment)
	add("track", m.EpisodeNumber)

	return tags
}

// EpisodeMetadata holds parsed episode information from Hugo frontmatter
type EpisodeMetadata struct {
	Episode         string    `yaml:"episode"`
	Title           string    `yaml:"title"`
	Date            time.Time `yaml:"Date"`
	EpisodeImage    string    `yaml:"episode_image"`
	PodcastDuration string    `yaml:"podcast_duration"`
	PodcastBytes    int64     `yaml:"podcast_bytes"`
}

// UnmarshalYAML decodes EpisodeMetadata while accepting either the capitalised
// "Date" key (used by all existing frontmatter) or a lowercase "date" key.
// yaml.v3 matches struct tags case-sensitively, so without this a lowercase
// "date:" would silently parse to the zero time.Time and produce a wrong date
// tag with no error surfaced. "Date" takes precedence when both appear.
func (m *EpisodeMetadata) UnmarshalYAML(value *yaml.Node) error {
	// Alias avoids infinite recursion into this method while reusing the tags.
	type rawMetadata EpisodeMetadata
	var raw rawMetadata
	if err := value.Decode(&raw); err != nil {
		return fmt.Errorf("failed to decode episode metadata: %w", err)
	}

	// rawMetadata only matches "Date"; decode the lowercase fallback separately.
	var lowercase struct {
		Date time.Time `yaml:"date"`
	}
	if err := value.Decode(&lowercase); err != nil {
		return fmt.Errorf("failed to decode episode date: %w", err)
	}

	if raw.Date.IsZero() {
		raw.Date = lowercase.Date
	}

	*m = EpisodeMetadata(raw)
	return nil
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
	if _, err := ParseEpisodeNumber(meta.Episode); err != nil {
		return nil, fmt.Errorf("invalid episode field: %w", err)
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
		return verifyCoverArtPath(coverPath)
	}

	// Otherwise the path is rooted at the Hugo site, served from static/.
	projectRoot, err := findProjectRoot(markdownDir)
	if err != nil {
		return "", err
	}

	coverPath := filepath.Join(projectRoot, "static", strings.TrimPrefix(episodeImage, "/"))
	return verifyCoverArtPath(coverPath)
}

func verifyCoverArtPath(coverPath string) (string, error) {
	coverPath, err := filepath.Abs(coverPath)
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

// FormatDateForID3 formats a time.Time to "YYYY-MM" for the muxer date tag
// (ID3 TDRC for MP3, and the equivalent date field in other muxers).
func FormatDateForID3(t time.Time) string {
	return t.Format("2006-01")
}

// UpdateFrontmatter updates podcast_duration and podcast_bytes in the markdown file
func UpdateFrontmatter(markdownPath, duration string, byteCount int64) error {
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	start, end, err := findFrontmatterBounds(lines)
	if err != nil {
		return fmt.Errorf("invalid frontmatter format: %w", err)
	}

	frontmatter := strings.Join(lines[start:end], "\n")
	updatedFrontmatter, err := updateFrontmatterYAML(frontmatter, duration, byteCount)
	if err != nil {
		return err
	}

	updatedLines := strings.Split(strings.TrimSuffix(updatedFrontmatter, "\n"), "\n")
	lines = slices.Replace(lines, start, end, updatedLines...)

	output := strings.Join(lines, "\n")
	if err := os.WriteFile(markdownPath, []byte(output), 0o644); err != nil { //nolint:gosec // markdownPath is user-provided input path, not tainted
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

func updateFrontmatterYAML(frontmatter, duration string, byteCount int64) (string, error) {
	var document yaml.Node
	if strings.TrimSpace(frontmatter) == "" {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	} else if err := yaml.Unmarshal([]byte(frontmatter), &document); err != nil {
		return "", fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	if len(document.Content) == 0 {
		document.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}

	mapping := document.Content[0]
	if mapping.Kind != yaml.MappingNode {
		return "", fmt.Errorf("invalid frontmatter: expected YAML mapping")
	}

	if !updateScalarField(mapping, "podcast_duration", duration, yaml.DoubleQuotedStyle, "!!str") {
		appendScalarField(mapping, "podcast_duration", duration, yaml.DoubleQuotedStyle, "!!str")
	}
	byteValue := strconv.FormatInt(byteCount, 10)
	if !updateScalarField(mapping, "podcast_bytes", byteValue, 0, "!!int") {
		appendScalarField(mapping, "podcast_bytes", byteValue, 0, "!!int")
	}

	var output bytes.Buffer
	encoder := yaml.NewEncoder(&output)
	encoder.SetIndent(2)
	if err := encoder.Encode(mapping); err != nil {
		return "", fmt.Errorf("failed to encode frontmatter: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return "", fmt.Errorf("failed to encode frontmatter: %w", err)
	}

	return output.String(), nil
}

func updateScalarField(mapping *yaml.Node, key, value string, style yaml.Style, tag string) bool {
	updated := false
	for i := 0; i+1 < len(mapping.Content); i += 2 {
		keyNode := mapping.Content[i]
		if keyNode.Kind != yaml.ScalarNode || keyNode.Value != key {
			continue
		}

		setScalarNode(mapping.Content[i+1], value, style, tag)
		updated = true
	}

	return updated
}

func appendScalarField(mapping *yaml.Node, key, value string, style yaml.Style, tag string) {
	mapping.Content = append(mapping.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value, Style: style},
	)
}

func setScalarNode(node *yaml.Node, value string, style yaml.Style, tag string) {
	node.Kind = yaml.ScalarNode
	node.Tag = tag
	node.Value = value
	node.Style = style
	node.Content = nil
	node.Anchor = ""
	node.Alias = nil
}
