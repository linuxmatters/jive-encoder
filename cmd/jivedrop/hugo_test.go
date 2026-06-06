package main

import (
	"strings"
	"testing"
)

// TestHugoWorkflowValidate tests Hugo mode validation of episode markdown arguments
func TestHugoWorkflowValidate(t *testing.T) {
	tests := []struct {
		name      string
		episodeMD string
		wantErr   bool
		errMatch  string // Substring to match in error message
	}{
		// Valid cases
		{
			name:      "valid markdown file lowercase .md",
			episodeMD: "episode.md",
			wantErr:   false,
		},
		{
			name:      "valid markdown file uppercase .MD",
			episodeMD: "episode.MD",
			wantErr:   false,
		},
		{
			name:      "valid markdown file mixed case .Md",
			episodeMD: "episode.Md",
			wantErr:   false,
		},
		{
			name:      "valid markdown with nested path",
			episodeMD: "content/episodes/67.md",
			wantErr:   false,
		},
		{
			name:      "valid markdown deeply nested",
			episodeMD: "posts/blog/2025/11/article.md",
			wantErr:   false,
		},
		{
			name:      "valid markdown with spaces in filename",
			episodeMD: "my episode file.md",
			wantErr:   false,
		},
		{
			name:      "valid markdown with special characters",
			episodeMD: "episode-67_final.md",
			wantErr:   false,
		},
		{
			name:      "valid markdown with multiple dots",
			episodeMD: "my.episode.v2.md",
			wantErr:   false,
		},

		// Invalid cases: missing EpisodeMD
		{
			name:      "empty episode markdown string",
			episodeMD: "",
			wantErr:   true,
			errMatch:  "requires episode markdown file",
		},

		// Invalid cases: wrong file extensions
		{
			name:      "wrong extension .txt",
			episodeMD: "episode.txt",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      "wrong extension .yaml",
			episodeMD: "episode.yaml",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      "wrong extension .json",
			episodeMD: "episode.json",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      "wrong extension .mp3",
			episodeMD: "episode.mp3",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},

		// Invalid cases: .md not at end
		{
			name:      ".md in middle of filename",
			episodeMD: "markdown_file.mp3",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      ".md with backup suffix",
			episodeMD: "episode.md.backup",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      ".md with bak extension",
			episodeMD: "episode.md.bak",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      ".md with old extension",
			episodeMD: "episode.md.old",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},

		// Edge cases
		{
			name:      "just .md filename",
			episodeMD: ".md",
			wantErr:   false,
		},
		{
			name:      "no extension",
			episodeMD: "episode",
			wantErr:   true,
			errMatch:  "must have .md extension",
		},
		{
			name:      "uppercase .MD only",
			episodeMD: ".MD",
			wantErr:   false,
		},
		{
			name:      "path with uppercase .MD",
			episodeMD: "content/episodes/POST.MD",
			wantErr:   false,
		},
		{
			name:      "relative path with ./",
			episodeMD: "./episode.md",
			wantErr:   false,
		},
		{
			name:      "relative path with ../",
			episodeMD: "../episodes/episode.md",
			wantErr:   false,
		},
		{
			name:      "absolute path (unix)",
			episodeMD: "/home/user/episodes/67.md",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct the workflow with the test inputs threaded through
			// CLIOptions. A dummy audio file keeps file-existence checks from
			// masking the argument validation errors we are testing for.
			wf := &HugoWorkflow{opts: CLIOptions{
				EpisodeMD: tt.episodeMD,
			}}
			err := wf.Validate()

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("HugoWorkflow.Validate() expected error, got nil (EpisodeMD=%q)", tt.episodeMD)
					return
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("HugoWorkflow.Validate() error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			// For valid cases we only check argument validation, not file existence.
			// File-not-found errors are acceptable here since the files do not exist on disk.
			if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not accessible") {
				t.Errorf("HugoWorkflow.Validate() unexpected error: %v (EpisodeMD=%q)", err, tt.episodeMD)
			}
		})
	}
}

// TestHugoWorkflowValidate_Integration tests HugoWorkflow.Validate with realistic scenarios
func TestHugoWorkflowValidate_Integration(t *testing.T) {
	tests := []struct {
		name        string
		episodeMD   string
		wantErr     bool
		description string
	}{
		{
			name:        "real hugo workflow file",
			episodeMD:   "content/episodes/67.md",
			wantErr:     false,
			description: "Typical Linux Matters episode markdown path",
		},
		{
			name:        "common user mistake: .txt instead of .md",
			episodeMD:   "episode.txt",
			wantErr:     true,
			description: "User accidentally passes wrong file type",
		},
		{
			name:        "common user mistake: no extension",
			episodeMD:   "episode",
			wantErr:     true,
			description: "User passes file without extension",
		},
		{
			name:        "common user mistake: .md.bak backup file",
			episodeMD:   "episode.md.bak",
			wantErr:     true,
			description: "User passes backup file instead of original",
		},
		{
			name:        "windows path with backslashes",
			episodeMD:   "content\\episodes\\67.md",
			wantErr:     false,
			description: "Cross-platform support for Windows paths",
		},
		{
			name:        "uppercase .MD extension",
			episodeMD:   "EPISODE.MD",
			wantErr:     false,
			description: "Case-insensitive extension matching",
		},
		{
			name:        "missing argument",
			episodeMD:   "",
			wantErr:     true,
			description: "User forgot to provide episode markdown file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Construct the workflow with the test inputs threaded through
			// CLIOptions. A dummy audio file keeps file-existence checks from
			// masking the argument validation errors we are testing for.
			wf := &HugoWorkflow{opts: CLIOptions{
				EpisodeMD: tt.episodeMD,
			}}
			err := wf.Validate()

			// Check error expectations
			if tt.wantErr && err == nil {
				t.Errorf("HugoWorkflow.Validate() expected error but got nil\n  Description: %s\n  EpisodeMD=%q",
					tt.description, tt.episodeMD)
			}
			if !tt.wantErr && err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not accessible") {
				t.Errorf("HugoWorkflow.Validate() unexpected error: %v\n  Description: %s\n  EpisodeMD=%q",
					err, tt.description, tt.episodeMD)
			}
		})
	}
}
