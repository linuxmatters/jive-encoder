package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSanitiseForFilename tests filename sanitisation for dangerous and special characters
func TestSanitiseForFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		// Basic cases
		{
			name:     "simple alphanumeric",
			input:    "Linux Matters",
			expected: "linux-matters",
		},
		// Special characters that should be removed
		{
			name:     "forward slash",
			input:    "AC/DC",
			expected: "acdc",
		},
		{
			name:     "backslash",
			input:    "Guns\\Roses",
			expected: "gunsroses",
		},
		{
			name:     "apostrophe",
			input:    "Guns N' Roses",
			expected: "guns-n-roses",
		},
		{
			name:     "ampersand",
			input:    "Tom & Jerry",
			expected: "tom--jerry",
		},
		{
			name:     "asterisk",
			input:    "The *Clash*",
			expected: "the-clash",
		},
		{
			name:     "question mark",
			input:    "Who?",
			expected: "who",
		},
		{
			name:     "exclamation mark",
			input:    "Bang!",
			expected: "bang",
		},
		// Unicode and non-ASCII characters
		{
			name:     "unicode umlauts stripped",
			input:    "Björk",
			expected: "bjrk",
		},
		{
			name:     "accented characters stripped",
			input:    "Café du Monde",
			expected: "caf-du-monde",
		},
		{
			name:     "chinese characters stripped",
			input:    "Podcast 中文 Show",
			expected: "podcast--show",
		},
		// Multiple special characters
		{
			name:     "multiple consecutive special chars",
			input:    "Episode!!!???",
			expected: "episode",
		},
		// Dots and underscores (preserved)
		{
			name:     "dots preserved",
			input:    "Hello...World",
			expected: "hello...world",
		},
		{
			name:     "underscores preserved",
			input:    "Hello_World",
			expected: "hello_world",
		},
		{
			name:     "mixed dots underscores hyphens",
			input:    "Hello.World_Test-Case",
			expected: "hello.world_test-case",
		},
		// Whitespace handling
		{
			name:     "leading and trailing spaces",
			input:    "  Podcast Show  ",
			expected: "--podcast-show--",
		},
		{
			name:     "multiple spaces between words",
			input:    "The   Podcast   Show",
			expected: "the---podcast---show",
		},
		{
			name:     "tabs and mixed whitespace",
			input:    "Hello\tWorld",
			expected: "helloworld",
		},
		// Edge cases
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only spaces",
			input:    "   ",
			expected: "---",
		},
		{
			name:     "only special characters",
			input:    "!!!???&&&",
			expected: "",
		},
		{
			name:     "numbers preserved",
			input:    "Episode 42",
			expected: "episode-42",
		},
		{
			name:     "mixed case with numbers",
			input:    "PoDCaSt 99 ShOw",
			expected: "podcast-99-show",
		},
		// Real-world examples
		{
			name:     "linux matters real example",
			input:    "Linux Matters",
			expected: "linux-matters",
		},
		{
			name:     "spotify podcast example",
			input:    "The Daily Show with Trevor Noah",
			expected: "the-daily-show-with-trevor-noah",
		},
		{
			name:     "special music artist",
			input:    "U2 / Bono",
			expected: "u2--bono",
		},
		{
			name:     "symbol-heavy artist",
			input:    "$$$ (Money) $$$",
			expected: "-money-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitiseForFilename(tt.input)
			if result != tt.expected {
				t.Errorf("sanitiseForFilename(%q) = %q; want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// TestGenerateFilename tests filename generation for both Hugo and Standalone modes
func TestGenerateFilename(t *testing.T) {
	tests := []struct {
		name      string
		mode      WorkflowMode
		num       string
		artist    string
		cliArtist string // Simulates CLI.Artist global
		expected  string
	}{
		// Hugo mode - default behaviour
		{
			name:      "hugo default simple",
			mode:      HugoMode,
			num:       "67",
			artist:    "",
			cliArtist: "",
			expected:  "LMP67.mp3",
		},
		{
			name:      "hugo episode 0",
			mode:      HugoMode,
			num:       "0",
			artist:    "",
			cliArtist: "",
			expected:  "LMP0.mp3",
		},
		{
			name:      "hugo large episode number",
			mode:      HugoMode,
			num:       "999",
			artist:    "",
			cliArtist: "",
			expected:  "LMP999.mp3",
		},
		// Hugo mode - custom artist override
		{
			name:      "hugo with custom artist override",
			mode:      HugoMode,
			num:       "67",
			artist:    "Custom Podcast",
			cliArtist: "Custom Podcast",
			expected:  "custom-podcast-67.mp3",
		},
		{
			name:      "hugo with special chars in artist",
			mode:      HugoMode,
			num:       "42",
			artist:    "The (Real) Show",
			cliArtist: "The (Real) Show",
			expected:  "the-real-show-42.mp3",
		},
		// Hugo mode - Linux Matters default not triggered by override
		{
			name:      "hugo with linux matters artist keeps default",
			mode:      HugoMode,
			num:       "50",
			artist:    "Linux Matters",
			cliArtist: "Linux Matters",
			expected:  "LMP50.mp3",
		},
		{
			name:      "hugo empty cli artist keeps default",
			mode:      HugoMode,
			num:       "55",
			artist:    "Other",
			cliArtist: "",
			expected:  "LMP55.mp3",
		},
		// Standalone mode - with artist
		{
			name:      "standalone with artist",
			mode:      StandaloneMode,
			num:       "1",
			artist:    "My Show",
			cliArtist: "My Show",
			expected:  "my-show-1.mp3",
		},
		{
			name:      "standalone with artist and special chars",
			mode:      StandaloneMode,
			num:       "42",
			artist:    "The Daily Show (Late Night)",
			cliArtist: "The Daily Show (Late Night)",
			expected:  "the-daily-show-late-night-42.mp3",
		},
		{
			name:      "standalone with multiple words",
			mode:      StandaloneMode,
			num:       "99",
			artist:    "This Is A Very Long Podcast Name",
			cliArtist: "This Is A Very Long Podcast Name",
			expected:  "this-is-a-very-long-podcast-name-99.mp3",
		},
		// Standalone mode - without artist (fallback to episode)
		{
			name:      "standalone without artist",
			mode:      StandaloneMode,
			num:       "1",
			artist:    "",
			cliArtist: "",
			expected:  "episode-1.mp3",
		},
		{
			name:      "standalone without artist large number",
			mode:      StandaloneMode,
			num:       "42",
			artist:    "",
			cliArtist: "",
			expected:  "episode-42.mp3",
		},
		// Edge cases with numbers
		{
			name:      "episode number with leading zeros",
			mode:      StandaloneMode,
			num:       "007",
			artist:    "James Bond",
			cliArtist: "James Bond",
			expected:  "james-bond-007.mp3",
		},
		{
			name:      "numeric artist name",
			mode:      StandaloneMode,
			num:       "5",
			artist:    "99 Luftballons",
			cliArtist: "99 Luftballons",
			expected:  "99-luftballons-5.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateFilename(tt.mode, tt.num, tt.artist, tt.cliArtist)
			if result != tt.expected {
				t.Errorf("generateFilename(%v, %q, %q, %q) = %q; want %q",
					tt.mode, tt.num, tt.artist, tt.cliArtist, result, tt.expected)
			}
		})
	}
}

// TestResolveOutputPath tests output path resolution with directories and files
func TestResolveOutputPath(t *testing.T) {
	tests := []struct {
		name       string
		outputPath string
		mode       WorkflowMode
		num        string
		artist     string
		cliArtist  string
		wantErr    bool
		wantPath   string // Substring check for path validation
	}{
		// Empty output path - use current directory with generated filename
		{
			name:       "empty path uses generated filename",
			outputPath: "",
			mode:       HugoMode,
			num:        "67",
			artist:     "",
			cliArtist:  "",
			wantErr:    false,
			wantPath:   "LMP67.mp3",
		},
		{
			name:       "empty path standalone mode",
			outputPath: "",
			mode:       StandaloneMode,
			num:        "42",
			artist:     "Test Show",
			cliArtist:  "Test Show",
			wantErr:    false,
			wantPath:   "test-show-42.mp3",
		},
		// Existing directory - generate filename within it
		{
			name:       "existing directory",
			outputPath: "", // Will be set to temp dir in test
			mode:       StandaloneMode,
			num:        "1",
			artist:     "Show",
			cliArtist:  "Show",
			wantErr:    false,
			wantPath:   "show-1.mp3",
		},
		// Explicit file path - use as-is
		{
			name:       "explicit filename in current dir",
			outputPath: "custom-output.mp3",
			mode:       StandaloneMode,
			num:        "1",
			artist:     "ignored",
			cliArtist:  "ignored",
			wantErr:    false,
			wantPath:   "custom-output.mp3",
		},
		// File path in existing directory
		{
			name:       "file path in existing temp directory",
			outputPath: "", // Will be set in test
			mode:       HugoMode,
			num:        "99",
			artist:     "",
			cliArtist:  "",
			wantErr:    false,
			wantPath:   "LMP99.mp3",
		},
		// Error cases: non-existent directory
		{
			name:       "trailing slash non-existent directory",
			outputPath: "/nonexistent/dir/",
			mode:       StandaloneMode,
			num:        "1",
			artist:     "test",
			cliArtist:  "test",
			wantErr:    true,
			wantPath:   "",
		},
		{
			name:       "file in non-existent directory",
			outputPath: "/nonexistent/deeply/nested/path/file.mp3",
			mode:       StandaloneMode,
			num:        "1",
			artist:     "test",
			cliArtist:  "test",
			wantErr:    true,
			wantPath:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle dynamic temp directory paths
			testOutputPath := tt.outputPath
			if tt.name == "existing directory" || tt.name == "file path in existing temp directory" {
				tmpDir := t.TempDir()
				testOutputPath = tmpDir
				if tt.name == "existing directory" {
					tt.wantPath = filepath.Join(testOutputPath, tt.wantPath)
				}
			}

			result, err := resolveOutputPath(tt.mode, tt.num, tt.artist, tt.cliArtist, testOutputPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("resolveOutputPath() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("resolveOutputPath() unexpected error: %v", err)
				return
			}

			// Check if result contains expected path component
			if tt.wantPath != "" && !isPathMatch(result, tt.wantPath) {
				t.Errorf("resolveOutputPath() = %q; want path containing %q", result, tt.wantPath)
			}
		})
	}
}

// TestResolveOutputPath_FileOverwrite tests file overwrite scenario
func TestResolveOutputPath_FileOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.mp3")

	// Create an existing file
	if err := os.WriteFile(existingFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Resolve the same path again
	result, err := resolveOutputPath(HugoMode, "1", "", "", existingFile)
	if err != nil {
		t.Errorf("resolveOutputPath() with existing file: got unexpected error: %v", err)
	}

	if result != existingFile {
		t.Errorf("resolveOutputPath() = %q; want %q", result, existingFile)
	}
}

// TestResolveOutputPath_GeneratedFilenameInTempDir tests generated filename placed in temp directory
func TestResolveOutputPath_GeneratedFilenameInTempDir(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := resolveOutputPath(StandaloneMode, "42", "Test Show", "Test Show", tmpDir)
	if err != nil {
		t.Errorf("resolveOutputPath() unexpected error: %v", err)
	}

	// Verify result is in the temp directory and has correct filename
	rel, relErr := filepath.Rel(tmpDir, result)
	if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		t.Errorf("resolveOutputPath() = %q; not in temp directory %q", result, tmpDir)
	}

	if !isPathMatch(result, "test-show-42.mp3") {
		t.Errorf("resolveOutputPath() = %q; want path containing 'test-show-42.mp3'", result)
	}
}

// isPathMatch checks if a path contains the expected component
// Handles both absolute and relative path matching
func isPathMatch(fullPath, expected string) bool {
	// Check if expected is at the end of the path (filename)
	if filepath.Base(fullPath) == expected {
		return true
	}
	// Check if expected is part of the path
	return strings.Contains(fullPath, expected)
}

// TestDetectMode tests the CLI mode detection logic for Hugo vs Standalone workflows
func TestDetectMode(t *testing.T) {
	tests := []struct {
		name      string
		audioFile string
		episodeMD string
		expected  WorkflowMode
	}{
		// Empty audio file - no arguments provided
		{
			name:      "empty audio file",
			audioFile: "",
			episodeMD: "",
			expected:  HugoMode, // Return value doesn't matter, exit will handle it
		},

		// Hugo mode: second argument is .md file
		{
			name:      "hugo mode with lowercase .md",
			audioFile: "podcast.flac",
			episodeMD: "episode.md",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with uppercase .MD",
			audioFile: "podcast.flac",
			episodeMD: "episode.MD",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with mixed case .Md",
			audioFile: "podcast.flac",
			episodeMD: "episode.Md",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with path containing .md",
			audioFile: "podcast.flac",
			episodeMD: "content/episodes/67.md",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with nested path and uppercase .MD",
			audioFile: "audio.wav",
			episodeMD: "posts/episode/post.MD",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with only filename .md",
			audioFile: "LMP67.flac",
			episodeMD: "67.md",
			expected:  HugoMode,
		},

		// Standalone mode: second argument is NOT a .md file
		{
			name:      "standalone mode with .txt file",
			audioFile: "podcast.flac",
			episodeMD: "readme.txt",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with .md in middle of filename",
			audioFile: "podcast.flac",
			episodeMD: "markdown_file.mp3",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with .md not at end",
			audioFile: "podcast.flac",
			episodeMD: "file.md.txt",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with empty episodeMD string",
			audioFile: "podcast.flac",
			episodeMD: "",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with audio file only",
			audioFile: "LMP67.flac",
			episodeMD: "",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with non-md extension",
			audioFile: "audio.wav",
			episodeMD: "episode.yaml",
			expected:  StandaloneMode,
		},

		// Edge cases
		{
			name:      "just .md (no filename before extension)",
			audioFile: "podcast.flac",
			episodeMD: ".md",
			expected:  HugoMode,
		},
		{
			name:      "filename with multiple dots ending in .md",
			audioFile: "podcast.flac",
			episodeMD: "my.episode.v2.md",
			expected:  HugoMode,
		},
		{
			name:      "filename with multiple dots not ending in .md",
			audioFile: "podcast.flac",
			episodeMD: "my.episode.v2.md.backup",
			expected:  StandaloneMode,
		},
		{
			name:      "markdown file with spaces",
			audioFile: "podcast.flac",
			episodeMD: "my episode file.md",
			expected:  HugoMode,
		},
		{
			name:      "episode md with special characters",
			audioFile: "podcast.flac",
			episodeMD: "episode-67_final.md",
			expected:  HugoMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMode(tt.audioFile, tt.episodeMD)

			// Verify result
			if result != tt.expected {
				t.Errorf("detectMode() = %v; want %v (AudioFile=%q, EpisodeMD=%q)",
					result, tt.expected, tt.audioFile, tt.episodeMD)
			}
		})
	}
}

// TestDetectMode_Integration tests detectMode in realistic scenarios
func TestDetectMode_Integration(t *testing.T) {
	tests := []struct {
		name        string
		audioFile   string
		episodeMD   string
		expected    WorkflowMode
		description string
	}{
		{
			name:        "real hugo workflow",
			audioFile:   "LMP67.flac",
			episodeMD:   "content/episodes/67.md",
			expected:    HugoMode,
			description: "User runs: jivedrop LMP67.flac content/episodes/67.md",
		},
		{
			name:        "real standalone workflow",
			audioFile:   "podcast.wav",
			episodeMD:   "",
			expected:    StandaloneMode,
			description: "User runs: jivedrop podcast.wav --title 'Ep 1' --num 1 --cover art.png",
		},
		{
			name:        "common mistake: user passes non-md file in hugo mode",
			audioFile:   "episode.flac",
			episodeMD:   "episode.txt",
			expected:    StandaloneMode,
			description: "User runs: jivedrop episode.flac episode.txt (should be .md not .txt)",
		},
		{
			name:        "edge: .md file with uppercase extension",
			audioFile:   "LMP99.flac",
			episodeMD:   "99.MD",
			expected:    HugoMode,
			description: "Handles .MD (uppercase) correctly for cross-platform",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMode(tt.audioFile, tt.episodeMD)

			// Verify result
			if result != tt.expected {
				t.Errorf("detectMode() = %v; want %v\n  Description: %s\n  AudioFile=%q, EpisodeMD=%q",
					result, tt.expected, tt.description, tt.audioFile, tt.episodeMD)
			}
		})
	}
}

// BenchmarkSanitiseForFilename benchmarks the sanitisation function
func BenchmarkSanitiseForFilename(b *testing.B) {
	testStrings := []string{
		"Linux Matters",
		"AC/DC",
		"The (Real) Show",
		"Podcast!!!???&&&",
		"Very Long Podcast Name With Many Words",
	}

	b.ResetTimer()
	for b.Loop() {
		for _, s := range testStrings {
			sanitiseForFilename(s)
		}
	}
}

// BenchmarkGenerateFilename benchmarks the filename generation
func BenchmarkGenerateFilename(b *testing.B) {
	b.ResetTimer()
	for b.Loop() {
		generateFilename(HugoMode, "67", "", "Linux Matters")
		generateFilename(StandaloneMode, "42", "My Podcast", "My Podcast")
		generateFilename(StandaloneMode, "1", "", "")
	}
}
