package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestStandaloneWorkflowValidate tests standalone mode validation of required flags.
func TestStandaloneWorkflowValidate(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		num      string
		cover    string
		wantErr  bool
		errMatch string // Substring to match in error message
	}{
		// Valid cases.
		{
			name:    "all flags present",
			title:   "Episode Title",
			num:     "1",
			cover:   "cover.png",
			wantErr: false,
		},
		{
			name:    "absolute cover path",
			title:   "Episode: The Quest (Part 1)",
			num:     "007",
			cover:   "/absolute/path/to/cover.png",
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
			name:    "cover path with spaces",
			title:   "Episode",
			num:     "42",
			cover:   "my cover art/final version.png",
			wantErr: false,
		},

		// Missing required flags.
		{
			name:     "missing title",
			title:    "",
			num:      "1",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "requires --title flag",
		},
		{
			name:     "missing num",
			title:    "Episode Title",
			num:      "",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "requires --num flag",
		},
		{
			name:     "missing cover",
			title:    "Episode Title",
			num:      "1",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --cover flag",
		},
		{
			name:     "all flags empty",
			title:    "",
			num:      "",
			cover:    "",
			wantErr:  true,
			errMatch: "requires --title flag",
		},

		// Edge cases.
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
		{
			name:     "num as negative (rejected)",
			title:    "Episode",
			num:      "-1",
			cover:    "cover.png",
			wantErr:  true,
			errMatch: "invalid --num flag",
		},
		{
			name:     "cover as url",
			title:    "Episode",
			num:      "1",
			cover:    "https://example.com/cover.png",
			wantErr:  true,
			errMatch: "cover art not accessible",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cover := tt.cover
			if !tt.wantErr {
				cover = existingCoverArgument(t, tt.cover)
			}

			wf := &StandaloneWorkflow{opts: CLIOptions{
				Title: tt.title,
				Num:   tt.num,
				Cover: cover,
			}}
			err := wf.Validate()

			if tt.wantErr {
				if err == nil {
					t.Errorf("StandaloneWorkflow.Validate() expected error, got nil\n  Title=%q, Num=%q, Cover=%q",
						tt.title, tt.num, cover)
					return
				}
				if tt.errMatch != "" && !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("StandaloneWorkflow.Validate() error %q does not contain %q", err.Error(), tt.errMatch)
				}
				return
			}

			if err != nil {
				t.Errorf("StandaloneWorkflow.Validate() unexpected error: %v\n  Title=%q, Num=%q, Cover=%q",
					err, tt.title, tt.num, cover)
			}
		})
	}
}

func existingCoverArgument(t *testing.T, path string) string {
	t.Helper()

	root := t.TempDir()
	workDir := filepath.Join(root, "work")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatalf("create test work dir: %v", err)
	}
	t.Chdir(workDir)

	arg := path
	if filepath.IsAbs(path) {
		arg = filepath.Join(root, strings.TrimPrefix(path, string(filepath.Separator)))
	}

	target := arg
	if !filepath.IsAbs(target) {
		target = filepath.Join(workDir, target)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("create cover fixture dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("cover"), 0o644); err != nil {
		t.Fatalf("create cover fixture: %v", err)
	}

	return arg
}
