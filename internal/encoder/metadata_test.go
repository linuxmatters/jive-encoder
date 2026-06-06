package encoder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseEpisodeMetadata(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid frontmatter",
			content: `---
episode: "67"
title: "Mirrors, Motors and Makefiles"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/episode/linuxmatters-3000x3000.png"
---

Episode content here.
`,
			wantErr: false,
		},
		{
			name: "missing episode field",
			content: `---
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---
`,
			wantErr:     true,
			errContains: "missing required field: episode",
		},
		{
			name: "missing title field",
			content: `---
episode: "67"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---
`,
			wantErr:     true,
			errContains: "missing required field: title",
		},
		{
			name: "missing episode_image field",
			content: `---
episode: "67"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
---
`,
			wantErr:     true,
			errContains: "missing required field: episode_image",
		},
		{
			name: "invalid YAML",
			content: `---
episode: "67"
title: [invalid: yaml: syntax
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---
`,
			wantErr:     true,
			errContains: "failed to parse frontmatter",
		},
		{
			name: "missing closing delimiter",
			content: `---
episode: "67"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"

No closing delimiter
`,
			wantErr:     true,
			errContains: "invalid frontmatter",
		},
		{
			name:        "no frontmatter delimiters",
			content:     "Just plain content without frontmatter\n",
			wantErr:     true,
			errContains: "invalid frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary test file
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Parse the metadata
			meta, err := ParseEpisodeMetadata(tmpFile)

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error containing '%s', got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errContains, err)
				}
				return
			}

			// Check success expectations
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if meta == nil {
				t.Error("Expected metadata, got nil")
			}
		})
	}
}

func TestParseEpisodeMetadata_ValidFields(t *testing.T) {
	content := `---
episode: "67"
title: "Mirrors, Motors and Makefiles"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/episode/linuxmatters-3000x3000.png"
podcast_duration: "54:09"
podcast_bytes: 27357184
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	meta, err := ParseEpisodeMetadata(tmpFile)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify all fields
	if meta.Episode != "67" {
		t.Errorf("Expected episode '67', got '%s'", meta.Episode)
	}
	if meta.Title != "Mirrors, Motors and Makefiles" {
		t.Errorf("Expected title 'Mirrors, Motors and Makefiles', got '%s'", meta.Title)
	}
	if meta.EpisodeImage != "/img/episode/linuxmatters-3000x3000.png" {
		t.Errorf("Expected episode_image '/img/episode/linuxmatters-3000x3000.png', got '%s'", meta.EpisodeImage)
	}
	if meta.PodcastDuration != "54:09" {
		t.Errorf("Expected podcast_duration '54:09', got '%s'", meta.PodcastDuration)
	}
	if meta.PodcastBytes != 27357184 {
		t.Errorf("Expected podcast_bytes 27357184, got %d", meta.PodcastBytes)
	}
}

func TestParseEpisodeNumber(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "plain number", input: "67", wantErr: false},
		{name: "zero", input: "0", wantErr: false},
		{name: "empty", input: "", wantErr: true},
		{name: "non-numeric", input: "foo", wantErr: true},
		{name: "trailing letter", input: "67a", wantErr: true},
		{name: "negative", input: "-1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseEpisodeNumber(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got nil", tt.input)
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", tt.input, err)
			}
			if got != tt.input {
				t.Errorf("Expected %q unchanged, got %q", tt.input, got)
			}
		})
	}
}

// TestUpdateFrontmatter_InsertBothFields tests inserting missing podcast_duration and podcast_bytes
func TestUpdateFrontmatter_InsertBothFields(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---

Episode content goes here.
More content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Update frontmatter with both fields missing
	err := UpdateFrontmatter(tmpFile, "01:23:45", 5555555)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify fields were added before closing delimiter
	if !contains(updatedContent, "podcast_duration: 01:23:45") {
		t.Error("podcast_duration field not found in updated frontmatter")
	}
	if !contains(updatedContent, "podcast_bytes: 5555555") {
		t.Error("podcast_bytes field not found in updated frontmatter")
	}

	// Verify fields are before the closing delimiter
	lines := splitLines(updatedContent)
	closingDelimiterIdx := -1
	durationIdx := -1
	bytesIdx := -1

	for i, line := range lines {
		if i > 0 && i < len(lines)-1 && line == "---" {
			closingDelimiterIdx = i
		}
		if contains(line, "podcast_duration:") {
			durationIdx = i
		}
		if contains(line, "podcast_bytes:") {
			bytesIdx = i
		}
	}

	if closingDelimiterIdx < 0 {
		t.Error("Closing delimiter not found")
	} else {
		if durationIdx < 0 || durationIdx >= closingDelimiterIdx {
			t.Error("podcast_duration not positioned before closing delimiter")
		}
		if bytesIdx < 0 || bytesIdx >= closingDelimiterIdx {
			t.Error("podcast_bytes not positioned before closing delimiter")
		}
	}

	// Verify episode body is preserved
	if !contains(updatedContent, "Episode content goes here.") {
		t.Error("Episode content was corrupted")
	}
	if !contains(updatedContent, "More content.") {
		t.Error("Episode content was corrupted")
	}
}

// TestUpdateFrontmatter_UpdateExistingFields tests updating fields that already exist
func TestUpdateFrontmatter_UpdateExistingFields(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
podcast_duration: "00:10:00"
podcast_bytes: 1000000
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Update with new values
	err := UpdateFrontmatter(tmpFile, "01:23:45", 5555555)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify fields were updated with new values
	if !contains(updatedContent, "podcast_duration: 01:23:45") {
		t.Error("podcast_duration not updated correctly")
	}
	if !contains(updatedContent, "podcast_bytes: 5555555") {
		t.Error("podcast_bytes not updated correctly")
	}

	// Verify old values are gone
	if contains(updatedContent, "podcast_duration: 00:10:00") {
		t.Error("Old podcast_duration value still present")
	}
	if contains(updatedContent, "podcast_bytes: 1000000") {
		t.Error("Old podcast_bytes value still present")
	}
}

// TestUpdateFrontmatter_InsertOneFieldUpdateOther tests inserting one field and updating the other
func TestUpdateFrontmatter_InsertOneFieldUpdateOther(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
podcast_duration: "00:05:00"
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Update - should update duration and insert bytes
	err := UpdateFrontmatter(tmpFile, "00:15:00", 3333333)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify both fields are now correct
	if !contains(updatedContent, "podcast_duration: 00:15:00") {
		t.Error("podcast_duration not updated")
	}
	if !contains(updatedContent, "podcast_bytes: 3333333") {
		t.Error("podcast_bytes not inserted")
	}
}

// TestUpdateFrontmatter_PreserveFrontmatterFields tests that other frontmatter fields are preserved
func TestUpdateFrontmatter_PreserveFrontmatterFields(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
summary: "A test episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
hosts:
 - alice
 - bob
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Update frontmatter
	err := UpdateFrontmatter(tmpFile, "00:20:00", 4444444)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify existing fields are preserved
	if !contains(updatedContent, "episode: \"42\"") {
		t.Error("episode field not preserved")
	}
	if !contains(updatedContent, "title: \"Test Episode\"") {
		t.Error("title field not preserved")
	}
	if !contains(updatedContent, "summary: \"A test episode\"") {
		t.Error("summary field not preserved")
	}
	if !contains(updatedContent, "Date: 2025-11-09T00:00:00Z") {
		t.Error("Date field not preserved")
	}
	if !contains(updatedContent, "hosts:") {
		t.Error("hosts field not preserved")
	}
	if !contains(updatedContent, " - alice") {
		t.Error("hosts list not preserved")
	}
	if !contains(updatedContent, " - bob") {
		t.Error("hosts list not preserved")
	}
}

// TestUpdateFrontmatter_InvalidFrontmatter tests error handling for invalid frontmatter
func TestUpdateFrontmatter_InvalidFrontmatter(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr bool
	}{
		{
			name:    "no frontmatter",
			content: "Just plain content without frontmatter\n",
			wantErr: true,
		},
		{
			name: "missing closing delimiter",
			content: `---
episode: "42"
title: "Test"
episode_image: "/img/test.png"
No closing delimiter
`,
			wantErr: true,
		},
		{
			name: "only opening delimiter",
			content: `---
Some content without proper closing
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			tmpFile := filepath.Join(tmpDir, "test.md")
			if err := os.WriteFile(tmpFile, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			err := UpdateFrontmatter(tmpFile, "00:10:00", 1000000)
			if !tt.wantErr && err != nil {
				t.Errorf("UpdateFrontmatter() unexpected error: %v", err)
			}
			if tt.wantErr && err == nil {
				t.Error("UpdateFrontmatter() expected error, got nil")
			}
		})
	}
}

// TestUpdateFrontmatter_LargeFileSize tests with large file size values
func TestUpdateFrontmatter_LargeFileSize(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Use a large file size (100 MB)
	largeSize := int64(100 * 1024 * 1024)

	err := UpdateFrontmatter(tmpFile, "54:32:10", largeSize)
	if err != nil {
		t.Fatalf("UpdateFrontmatter with large size failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	if !contains(updatedContent, "podcast_duration: 54:32:10") {
		t.Error("Duration not updated correctly")
	}
	if !contains(updatedContent, "podcast_bytes: 104857600") {
		t.Error("Large file size not updated correctly")
	}
}

// TestUpdateFrontmatter_ZeroDuration tests with zero duration
func TestUpdateFrontmatter_ZeroDuration(t *testing.T) {
	content := `---
episode: "0"
title: "Pilot"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Edge case: zero duration
	err := UpdateFrontmatter(tmpFile, "00:00:00", 0)
	if err != nil {
		t.Fatalf("UpdateFrontmatter with zero values failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	if !contains(updatedContent, "podcast_duration: 00:00:00") {
		t.Error("Zero duration not handled correctly")
	}
	if !contains(updatedContent, "podcast_bytes: 0") {
		t.Error("Zero bytes not handled correctly")
	}
}

// TestUpdateFrontmatter_FileWithNoBody tests frontmatter-only file
func TestUpdateFrontmatter_FileWithNoBody(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := UpdateFrontmatter(tmpFile, "00:10:00", 1000000)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify fields were added
	if !contains(updatedContent, "podcast_duration: 00:10:00") {
		t.Error("podcast_duration not added")
	}
	if !contains(updatedContent, "podcast_bytes: 1000000") {
		t.Error("podcast_bytes not added")
	}
}

// TestUpdateFrontmatter_MultilineFieldValues tests with multiline YAML fields
func TestUpdateFrontmatter_MultilineFieldValues(t *testing.T) {
	content := `---
episode: "42"
title: "Test Episode"
summary: |
  This is a multiline
  summary that spans
  multiple lines
Date: 2025-11-09T00:00:00Z
episode_image: "/img/test.png"
---

Episode content.
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	err := UpdateFrontmatter(tmpFile, "00:25:00", 2222222)
	if err != nil {
		t.Fatalf("UpdateFrontmatter failed: %v", err)
	}

	// Read the updated file
	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read updated file: %v", err)
	}

	updatedContent := string(updated)

	// Verify fields were added and multiline content preserved
	if !contains(updatedContent, "podcast_duration: 00:25:00") {
		t.Error("podcast_duration not added")
	}
	if !contains(updatedContent, "This is a multiline") {
		t.Error("Multiline content was corrupted")
	}
	if !contains(updatedContent, "multiple lines") {
		t.Error("Multiline content was corrupted")
	}
}

func TestResolveCoverArtPath_RelativePath(t *testing.T) {
	// Create a temporary directory structure:
	// tmpDir/episode.md
	// tmpDir/images/cover.png
	tmpDir := t.TempDir()

	// Create cover art file
	imagesDir := filepath.Join(tmpDir, "images")
	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		t.Fatalf("Failed to create images directory: %v", err)
	}

	coverPath := filepath.Join(imagesDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("fake png"), 0o644); err != nil {
		t.Fatalf("Failed to create cover art file: %v", err)
	}

	// Create episode markdown file
	markdownPath := filepath.Join(tmpDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "./images/cover.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution
	resolved, err := ResolveCoverArtPath(markdownPath, "./images/cover.png")
	if err != nil {
		t.Fatalf("ResolveCoverArtPath failed: %v", err)
	}

	absExpected, _ := filepath.Abs(coverPath)
	if resolved != absExpected {
		t.Errorf("Expected %s, got %s", absExpected, resolved)
	}
}

func TestResolveCoverArtPath_RelativePathSubdirectory(t *testing.T) {
	// Test relative path in nested subdirectory with ./ prefix
	tmpDir := t.TempDir()

	// Create nested structure:
	// tmpDir/content/episodes/ep1.md
	// tmpDir/content/episodes/cover.png
	nestedDir := filepath.Join(tmpDir, "content", "episodes")

	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	// Create cover art file in same directory as markdown
	coverPath := filepath.Join(nestedDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("fake png"), 0o644); err != nil {
		t.Fatalf("Failed to create cover art file: %v", err)
	}

	// Create episode markdown file
	markdownPath := filepath.Join(nestedDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "./cover.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution - ./ prefix means relative to markdown location
	resolved, err := ResolveCoverArtPath(markdownPath, "./cover.png")
	if err != nil {
		t.Fatalf("ResolveCoverArtPath failed: %v", err)
	}

	absExpected, _ := filepath.Abs(coverPath)
	if resolved != absExpected {
		t.Errorf("Expected %s, got %s", absExpected, resolved)
	}
}

func TestResolveCoverArtPath_AbsolutePath(t *testing.T) {
	// Create a temporary Hugo project structure:
	// tmpDir/static/img/cover.png
	// tmpDir/content/episodes/episode.md
	tmpDir := t.TempDir()

	// Create Hugo static directory structure
	staticDir := filepath.Join(tmpDir, "static", "img")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	// Create cover art file
	coverPath := filepath.Join(staticDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("fake png"), 0o644); err != nil {
		t.Fatalf("Failed to create cover art file: %v", err)
	}

	// Create episode markdown file in nested content directory
	contentDir := filepath.Join(tmpDir, "content", "episodes")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatalf("Failed to create content directory: %v", err)
	}

	markdownPath := filepath.Join(contentDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "/img/cover.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution - should walk up from content/episodes to find static/
	resolved, err := ResolveCoverArtPath(markdownPath, "/img/cover.png")
	if err != nil {
		t.Fatalf("ResolveCoverArtPath failed: %v", err)
	}

	absExpected, _ := filepath.Abs(coverPath)
	if resolved != absExpected {
		t.Errorf("Expected %s, got %s", absExpected, resolved)
	}
}

func TestResolveCoverArtPath_AbsolutePathNested(t *testing.T) {
	// Test absolute path with nested static subdirectories
	tmpDir := t.TempDir()

	// Create structure:
	// tmpDir/static/media/podcasts/cover.png
	// tmpDir/content/blog/post.md
	staticDir := filepath.Join(tmpDir, "static", "media", "podcasts")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	coverPath := filepath.Join(staticDir, "cover.png")
	if err := os.WriteFile(coverPath, []byte("fake png"), 0o644); err != nil {
		t.Fatalf("Failed to create cover art file: %v", err)
	}

	contentDir := filepath.Join(tmpDir, "content", "blog")
	if err := os.MkdirAll(contentDir, 0o755); err != nil {
		t.Fatalf("Failed to create content directory: %v", err)
	}

	markdownPath := filepath.Join(contentDir, "post.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "/media/podcasts/cover.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution
	resolved, err := ResolveCoverArtPath(markdownPath, "/media/podcasts/cover.png")
	if err != nil {
		t.Fatalf("ResolveCoverArtPath failed: %v", err)
	}

	absExpected, _ := filepath.Abs(coverPath)
	if resolved != absExpected {
		t.Errorf("Expected %s, got %s", absExpected, resolved)
	}
}

func TestResolveCoverArtPath_FileNotFound_Relative(t *testing.T) {
	// Test error when relative path file does not exist
	tmpDir := t.TempDir()

	markdownPath := filepath.Join(tmpDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "./missing.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution - should fail
	_, err := ResolveCoverArtPath(markdownPath, "./missing.png")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
	if !contains(err.Error(), "cover art not found") {
		t.Errorf("Expected 'cover art not found' error, got: %v", err)
	}
}

func TestResolveCoverArtPath_FileNotFound_Absolute(t *testing.T) {
	// Test error when absolute path file does not exist
	tmpDir := t.TempDir()

	// Create Hugo structure without the cover file
	staticDir := filepath.Join(tmpDir, "static")
	if err := os.MkdirAll(staticDir, 0o755); err != nil {
		t.Fatalf("Failed to create static directory: %v", err)
	}

	markdownPath := filepath.Join(tmpDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "/img/missing.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution - should fail
	_, err := ResolveCoverArtPath(markdownPath, "/img/missing.png")
	if err == nil {
		t.Error("Expected error for missing file, got nil")
	}
	if !contains(err.Error(), "cover art not found") {
		t.Errorf("Expected 'cover art not found' error, got: %v", err)
	}
}

func TestResolveCoverArtPath_NoProjectRoot(t *testing.T) {
	// Test error when no Hugo project root (no static/ directory) is found
	tmpDir := t.TempDir()

	// Create markdown file without any static/ directory in parent hierarchy
	markdownPath := filepath.Join(tmpDir, "episode.md")
	markdownContent := `---
episode: "1"
title: "Test"
episode_image: "/img/cover.png"
---
`
	if err := os.WriteFile(markdownPath, []byte(markdownContent), 0o644); err != nil {
		t.Fatalf("Failed to create markdown file: %v", err)
	}

	// Test resolution - should fail
	_, err := ResolveCoverArtPath(markdownPath, "/img/cover.png")
	if err == nil {
		t.Error("Expected error for missing project root, got nil")
	}
	if !contains(err.Error(), "could not find Hugo project root") {
		t.Errorf("Expected 'could not find Hugo project root' error, got: %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// splitLines splits content by newline for position analysis
func splitLines(content string) []string {
	return strings.Split(content, "\n")
}
