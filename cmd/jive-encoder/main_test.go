package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
)

// TestSanitiseForFilename tests filename sanitisation for dangerous and special characters
func TestSanitiseForFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "simple alphanumeric",
			input:    "Linux Matters",
			expected: "linux-matters",
		},
		{
			name:     "strips punctuation",
			input:    "AC/DC & Friends!",
			expected: "acdc--friends",
		},
		{
			name:     "strips unicode",
			input:    "Café 中文 Show",
			expected: "caf--show",
		},
		{
			name:     "preserves dots underscores and hyphens",
			input:    "Hello.World_Test-Case",
			expected: "hello.world_test-case",
		},
		{
			name:     "spaces become hyphens",
			input:    "  The   Podcast  ",
			expected: "--the---podcast--",
		},
		{
			name:     "tabs are stripped",
			input:    "Hello\tWorld",
			expected: "helloworld",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
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
		ext       string
		expected  string
	}{
		{
			name:      "hugo default simple",
			mode:      HugoMode,
			num:       "67",
			artist:    "",
			cliArtist: "",
			ext:       ".mp3",
			expected:  "LMP67.mp3",
		},
		{
			name:      "hugo default opus",
			mode:      HugoMode,
			num:       "67",
			artist:    "",
			cliArtist: "",
			ext:       ".opus",
			expected:  "LMP67.opus",
		},
		{
			name:      "standalone artist opus",
			mode:      StandaloneMode,
			num:       "1",
			artist:    "My Show",
			cliArtist: "My Show",
			ext:       ".opus",
			expected:  "my-show-1.opus",
		},
		{
			name:      "hugo episode 0",
			mode:      HugoMode,
			num:       "0",
			artist:    "",
			cliArtist: "",
			ext:       ".mp3",
			expected:  "LMP0.mp3",
		},
		{
			name:      "hugo with custom artist override",
			mode:      HugoMode,
			num:       "67",
			artist:    "Custom Podcast",
			cliArtist: "Custom Podcast",
			ext:       ".mp3",
			expected:  "custom-podcast-67.mp3",
		},
		{
			name:      "hugo with linux matters artist keeps default",
			mode:      HugoMode,
			num:       "50",
			artist:    "Linux Matters",
			cliArtist: "Linux Matters",
			ext:       ".mp3",
			expected:  "LMP50.mp3",
		},
		{
			name:      "hugo empty cli artist keeps default",
			mode:      HugoMode,
			num:       "55",
			artist:    "Other",
			cliArtist: "",
			ext:       ".mp3",
			expected:  "LMP55.mp3",
		},
		{
			name:      "standalone with artist",
			mode:      StandaloneMode,
			num:       "1",
			artist:    "My Show",
			cliArtist: "My Show",
			ext:       ".mp3",
			expected:  "my-show-1.mp3",
		},
		{
			name:      "standalone with artist and special chars",
			mode:      StandaloneMode,
			num:       "42",
			artist:    "The Daily Show (Late Night)",
			cliArtist: "The Daily Show (Late Night)",
			ext:       ".mp3",
			expected:  "the-daily-show-late-night-42.mp3",
		},
		{
			name:      "standalone without artist",
			mode:      StandaloneMode,
			num:       "1",
			artist:    "",
			cliArtist: "",
			ext:       ".mp3",
			expected:  "episode-1.mp3",
		},
		{
			name:      "episode number with leading zeros",
			mode:      StandaloneMode,
			num:       "007",
			artist:    "James Bond",
			cliArtist: "James Bond",
			ext:       ".mp3",
			expected:  "james-bond-007.mp3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := generateFilename(tt.mode, tt.num, tt.artist, tt.cliArtist, tt.ext)
			if result != tt.expected {
				t.Errorf("generateFilename(%v, %q, %q, %q, %q) = %q; want %q",
					tt.mode, tt.num, tt.artist, tt.cliArtist, tt.ext, result, tt.expected)
			}
		})
	}
}

// TestResolveOutputPath tests output path resolution with directories and files
func TestResolveOutputPath(t *testing.T) {
	tests := []struct {
		name          string
		outputPath    string
		mode          WorkflowMode
		num           string
		artist        string
		cliArtist     string
		ext           string
		wantErr       bool
		wantPath      string // Substring check for path validation
		useTempDir    bool   // Replace outputPath with a fresh temp directory
		wantInTempDir bool   // Prefix wantPath with the temp directory
	}{
		{
			name:       "empty path uses generated filename",
			outputPath: "",
			mode:       HugoMode,
			num:        "67",
			artist:     "",
			cliArtist:  "",
			ext:        ".mp3",
			wantErr:    false,
			wantPath:   "LMP67.mp3",
		},
		{
			name:       "empty path opus extension",
			outputPath: "",
			mode:       HugoMode,
			num:        "67",
			artist:     "",
			cliArtist:  "",
			ext:        ".opus",
			wantErr:    false,
			wantPath:   "LMP67.opus",
		},
		{
			name:          "existing directory",
			outputPath:    "", // Replaced with temp dir via useTempDir
			mode:          StandaloneMode,
			num:           "1",
			artist:        "Show",
			cliArtist:     "Show",
			ext:           ".mp3",
			wantErr:       false,
			wantPath:      "show-1.mp3",
			useTempDir:    true,
			wantInTempDir: true,
		},
		{
			name:       "explicit filename in current dir",
			outputPath: "custom-output.mp3",
			mode:       StandaloneMode,
			num:        "1",
			artist:     "ignored",
			cliArtist:  "ignored",
			ext:        ".mp3",
			wantErr:    false,
			wantPath:   "custom-output.mp3",
		},
		{
			name:       "trailing slash non-existent directory",
			outputPath: "/nonexistent/dir/",
			mode:       StandaloneMode,
			num:        "1",
			artist:     "test",
			cliArtist:  "test",
			ext:        ".mp3",
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
			ext:        ".mp3",
			wantErr:    true,
			wantPath:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Handle dynamic temp directory paths
			testOutputPath := tt.outputPath
			wantPath := tt.wantPath
			if tt.useTempDir {
				testOutputPath = t.TempDir()
				if tt.wantInTempDir {
					wantPath = filepath.Join(testOutputPath, wantPath)
				}
			}

			result, err := resolveOutputPath(tt.mode, tt.num, tt.artist, tt.cliArtist, tt.ext, testOutputPath)

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

			if wantPath != "" && !isPathMatch(result, wantPath) {
				t.Errorf("resolveOutputPath() = %q; want path containing %q", result, wantPath)
			}
		})
	}
}

// TestResolveOutputPath_FileOverwrite tests file overwrite scenario
func TestResolveOutputPath_FileOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.mp3")

	if err := os.WriteFile(existingFile, []byte("dummy"), 0o644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	result, err := resolveOutputPath(HugoMode, "1", "", "", ".mp3", existingFile)
	if err != nil {
		t.Errorf("resolveOutputPath() with existing file: got unexpected error: %v", err)
	}

	if result != existingFile {
		t.Errorf("resolveOutputPath() = %q; want %q", result, existingFile)
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
		episodeMD string
		expected  WorkflowMode
	}{
		{
			name:      "hugo mode with lowercase .md",
			episodeMD: "episode.md",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with uppercase .MD",
			episodeMD: "episode.MD",
			expected:  HugoMode,
		},
		{
			name:      "hugo mode with path containing .md",
			episodeMD: "content/episodes/67.md",
			expected:  HugoMode,
		},
		{
			name:      "standalone mode with .txt file",
			episodeMD: "readme.txt",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with .md in middle of filename",
			episodeMD: "markdown_file.mp3",
			expected:  StandaloneMode,
		},
		{
			name:      "standalone mode with empty episodeMD string",
			episodeMD: "",
			expected:  StandaloneMode,
		},
		{
			name:      "just .md (no filename before extension)",
			episodeMD: ".md",
			expected:  HugoMode,
		},
		{
			name:      "filename with multiple dots ending in .md",
			episodeMD: "my.episode.v2.md",
			expected:  HugoMode,
		},
		{
			name:      "filename with multiple dots not ending in .md",
			episodeMD: "my.episode.v2.md.backup",
			expected:  StandaloneMode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectMode(tt.episodeMD)

			if result != tt.expected {
				t.Errorf("detectMode() = %v; want %v (EpisodeMD=%q)",
					result, tt.expected, tt.episodeMD)
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
		generateFilename(HugoMode, "67", "", "Linux Matters", ".mp3")
		generateFilename(StandaloneMode, "42", "My Podcast", "My Podcast", ".mp3")
		generateFilename(StandaloneMode, "1", "", "", ".mp3")
	}
}

// TestFormatFlag verifies the --format Kong enum accepts the three supported
// formats, rejects unknown values at parse time, and defaults to mp3.
func TestFormatFlag(t *testing.T) {
	type formatCLI struct {
		Format string `enum:"mp3,opus,aac" default:"mp3"`
	}

	parse := func(args []string) (string, error) {
		var c formatCLI
		parser, err := kong.New(&c)
		if err != nil {
			t.Fatalf("failed to build parser: %v", err)
		}
		if _, err := parser.Parse(args); err != nil {
			return "", err
		}
		return c.Format, nil
	}

	t.Run("opus accepted", func(t *testing.T) {
		got, err := parse([]string{"--format", "opus"})
		if err != nil {
			t.Fatalf("expected --format opus to parse, got error: %v", err)
		}
		if got != "opus" {
			t.Fatalf("expected format opus, got %q", got)
		}
	})

	t.Run("flac rejected", func(t *testing.T) {
		if _, err := parse([]string{"--format", "flac"}); err == nil {
			t.Fatal("expected --format flac to be rejected by the enum")
		}
	})

	t.Run("defaults to mp3", func(t *testing.T) {
		got, err := parse(nil)
		if err != nil {
			t.Fatalf("expected default invocation to parse, got error: %v", err)
		}
		if got != "mp3" {
			t.Fatalf("expected default format mp3, got %q", got)
		}
	})
}
