package main

import (
	"strings"
	"testing"
)

// TestStandaloneWorkflowValidate tests standalone mode validation of required flags
func TestStandaloneWorkflowValidate(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		num      string
		cover    string
		wantErr  bool
		errMatch string // Substring to match in error message
	}{
		// Valid cases: all flags present
		{
			name:    "all flags present",
			title:   "Episode Title",
			num:     "1",
			cover:   "cover.png",
			wantErr: false,
		},
		{
			name:    "all flags with special characters",
			title:   "Episode: The Quest (Part 1)",
			num:     "42",
			cover:   "artwork/cover-2025.png",
			wantErr: false,
		},
		{
			name:    "all flags with long values",
			title:   "This is a very long episode title with lots of words and details",
			num:     "999",
			cover:   "/absolute/path/to/very/deep/directory/structure/cover.png",
			wantErr: false,
		},
		{
			name:    "all flags with minimal values",
			title:   "A",
			num:     "0",
			cover:   "c.png",
			wantErr: false,
		},
		{
			name:    "all flags with unicode in title",
			title:   "Podcast — Episode 1",
			num:     "1",
			cover:   "cover.png",
			wantErr: false,
		},

		// Invalid cases: missing title
		{
			name:     "missing title",
			title:    "",
			num:      "1",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "requires --title flag",
		},
		{
			name:     "missing title with num and cover",
			title:    "",
			num:      "42",
			cover:    "/path/to/cover.png",
			wantErr:  true,
			errMatch: "requires --title flag",
		},

		// Invalid cases: missing num
		{
			name:     "missing num",
			title:    "Episode Title",
			num:      "",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "requires --num flag",
		},
		{
			name:     "missing num with title and cover",
			title:    "My Show",
			num:      "",
			cover:    "artwork.png",
			wantErr:  true,
			errMatch: "requires --num flag",
		},

		// Invalid cases: missing cover
		{
			name:     "missing cover",
			title:    "Episode Title",
			num:      "1",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --cover flag",
		},
		{
			name:     "missing cover with title and num",
			title:    "My Podcast",
			num:      "100",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --cover flag",
		},

		// Invalid cases: multiple flags missing
		{
			name:     "missing title and num",
			title:    "",
			num:      "",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "requires --title flag", // First validation fails
		},
		{
			name:     "missing title and cover",
			title:    "",
			num:      "1",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --title flag", // First validation fails
		},
		{
			name:     "missing num and cover",
			title:    "Episode",
			num:      "",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --num flag", // First validation fails
		},
		{
			name:     "all flags empty",
			title:    "",
			num:      "",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --title flag", // First validation fails
		},

		// Edge cases: whitespace-only values (still invalid if they parse as empty)
		{
			name:    "whitespace-only title (not trimmed by validator)",
			title:   "   ",
			num:     "1",
			cover:   "cover.png",
			wantErr: false, // Not empty string, so passes validation
		},
		{
			name:     "whitespace-only num (rejected as non-numeric)",
			title:    "Episode",
			num:      "   ",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "invalid --num flag",
		},

		// Edge cases: numeric-like values
		{
			name:    "num as string with leading zeros",
			title:   "Episode",
			num:     "007",
			cover:   "cover.png",
			wantErr: false,
		},
		{
			name:     "num as negative (rejected)",
			title:    "Episode",
			num:      "-1",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "invalid --num flag",
		},
		{
			name:     "num as decimal (rejected as non-integer)",
			title:    "Episode",
			num:      "1.5",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "invalid --num flag",
		},

		// Cover art path variations
		{
			name:    "cover as url",
			title:   "Episode",
			num:     "1",
			cover:   "https://example.com/cover.png",
			wantErr: false,
		},
		{
			name:    "cover as relative path",
			title:   "Episode",
			num:     "1",
			cover:   "./images/cover.png",
			wantErr: false,
		},
		{
			name:    "cover as absolute path",
			title:   "Episode",
			num:     "1",
			cover:   "/absolute/path/cover.png",
			wantErr: false,
		},
		{
			name:    "cover with spaces in path",
			title:   "Episode",
			num:     "1",
			cover:   "my cover art/final version.png",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global state
			originalTitle := CLI.Title
			originalNum := CLI.Num
			originalCover := CLI.Cover
			originalAudioFile := CLI.AudioFile
			defer func() {
				CLI.Title = originalTitle
				CLI.Num = originalNum
				CLI.Cover = originalCover
				CLI.AudioFile = originalAudioFile
			}()

			// Set test values
			CLI.Title = tt.title
			CLI.Num = tt.num
			CLI.Cover = tt.cover
			// Set a dummy audio file so file-existence checks do not mask
			// the argument validation errors we are testing for.
			CLI.AudioFile = "testdata"

			// Call Validate on StandaloneWorkflow
			wf := &StandaloneWorkflow{}
			err := wf.Validate()

			// Check error expectations
			if tt.wantErr {
				if err == nil {
					t.Errorf("StandaloneWorkflow.Validate() expected error, got nil\n  Title=%q, Num=%q, Cover=%q",
						tt.title, tt.num, tt.cover)
					return
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("StandaloneWorkflow.Validate() error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			// For valid cases we only check argument validation, not file existence.
			// File-not-found errors are acceptable here since the cover files do not exist on disk.
			if err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not accessible") {
				t.Errorf("StandaloneWorkflow.Validate() unexpected error: %v\n  Title=%q, Num=%q, Cover=%q",
					err, tt.title, tt.num, tt.cover)
			}
		})
	}
}

// TestStandaloneWorkflowValidate_Integration tests StandaloneWorkflow.Validate with realistic scenarios
func TestStandaloneWorkflowValidate_Integration(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		num         string
		cover       string
		wantErr     bool
		description string
	}{
		{
			name:        "valid standalone workflow",
			title:       "Terminal Full of Sparkles",
			num:         "66",
			cover:       "artwork.png",
			wantErr:     false,
			description: "User runs: jivedrop audio.flac --title 'Terminal Full of Sparkles' --num 66 --cover artwork.png",
		},
		{
			name:        "common mistake: forgot --title flag",
			title:       "",
			num:         "42",
			cover:       "cover.png",
			wantErr:     true,
			description: "User forgets required --title flag",
		},
		{
			name:        "common mistake: forgot --num flag",
			title:       "My Episode",
			num:         "",
			cover:       "cover.png",
			wantErr:     true,
			description: "User forgets required --num flag (episode number)",
		},
		{
			name:        "common mistake: forgot --cover flag",
			title:       "My Episode",
			num:         "1",
			cover:       "",
			wantErr:     true,
			description: "User forgets required --cover flag",
		},
		{
			name:        "forgot all flags",
			title:       "",
			num:         "",
			cover:       "",
			wantErr:     true,
			description: "User forgets all required flags",
		},
		{
			name:        "minimal valid values",
			title:       "Ep",
			num:         "1",
			cover:       "art.png",
			wantErr:     false,
			description: "Minimal valid values for standalone mode",
		},
		{
			name:        "episode with complex path",
			title:       "The Daily Show",
			num:         "99",
			cover:       "/home/user/podcasts/2025/daily-show-99.png",
			wantErr:     false,
			description: "Real-world example with full path to cover",
		},
		{
			name:        "missing only cover field",
			title:       "Podcast Name",
			num:         "50",
			cover:       "",
			wantErr:     true,
			description: "Most common mistake: user specifies --title and --num but forgets --cover",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore global state
			originalTitle := CLI.Title
			originalNum := CLI.Num
			originalCover := CLI.Cover
			originalAudioFile := CLI.AudioFile
			defer func() {
				CLI.Title = originalTitle
				CLI.Num = originalNum
				CLI.Cover = originalCover
				CLI.AudioFile = originalAudioFile
			}()

			// Set test values
			CLI.Title = tt.title
			CLI.Num = tt.num
			CLI.Cover = tt.cover
			CLI.AudioFile = "testdata"

			// Call Validate on StandaloneWorkflow
			wf := &StandaloneWorkflow{}
			err := wf.Validate()

			// Check error expectations
			if tt.wantErr && err == nil {
				t.Errorf("StandaloneWorkflow.Validate() expected error but got nil\n  Description: %s\n  Title=%q, Num=%q, Cover=%q",
					tt.description, tt.title, tt.num, tt.cover)
			}
			if !tt.wantErr && err != nil && !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "not accessible") {
				t.Errorf("StandaloneWorkflow.Validate() unexpected error: %v\n  Description: %s\n  Title=%q, Num=%q, Cover=%q",
					err, tt.description, tt.title, tt.num, tt.cover)
			}
		})
	}
}
