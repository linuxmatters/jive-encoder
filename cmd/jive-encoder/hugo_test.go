package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHugoWorkflowValidate tests Hugo mode validation of episode markdown arguments.
func TestHugoWorkflowValidate(t *testing.T) {
	t.Run("requires episode markdown", func(t *testing.T) {
		wf := &HugoWorkflow{opts: CLIOptions{}}

		err := wf.Validate()
		if err == nil {
			t.Fatal("HugoWorkflow.Validate() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "requires episode markdown file") {
			t.Errorf("HugoWorkflow.Validate() error %q does not contain %q", err.Error(), "requires episode markdown file")
		}
	})

	t.Run("rejects non-markdown paths", func(t *testing.T) {
		invalidPaths := []string{
			"episode.txt",
			"episode",
			"episode.md.bak",
			"markdown_file.mp3",
		}

		for _, episodeMD := range invalidPaths {
			t.Run(episodeMD, func(t *testing.T) {
				wf := &HugoWorkflow{opts: CLIOptions{
					EpisodeMD: episodeMD,
				}}

				err := wf.Validate()
				if err == nil {
					t.Fatalf("HugoWorkflow.Validate() expected error, got nil (EpisodeMD=%q)", episodeMD)
				}
				if !strings.Contains(err.Error(), "must have .md extension") {
					t.Errorf("HugoWorkflow.Validate() error %q does not contain %q", err.Error(), "must have .md extension")
				}
			})
		}
	})

	t.Run("rejects inaccessible markdown file", func(t *testing.T) {
		wf := &HugoWorkflow{opts: CLIOptions{
			EpisodeMD: filepath.Join(t.TempDir(), "missing.md"),
		}}

		err := wf.Validate()
		if err == nil {
			t.Fatal("HugoWorkflow.Validate() expected error, got nil")
		}
		if !strings.Contains(err.Error(), "episode file not accessible") {
			t.Errorf("HugoWorkflow.Validate() error %q does not contain %q", err.Error(), "episode file not accessible")
		}
	})

	t.Run("accepts existing markdown paths", func(t *testing.T) {
		validPaths := []string{
			"content/episodes/67.md",
			"EPISODE.MD",
			"episode.Md",
			"content\\episodes\\67.md",
			"/home/user/episodes/67.md",
		}

		for _, path := range validPaths {
			t.Run(path, func(t *testing.T) {
				episodeMD := existingMarkdownArgument(t, path)
				wf := &HugoWorkflow{opts: CLIOptions{
					EpisodeMD: episodeMD,
				}}

				if err := wf.Validate(); err != nil {
					t.Errorf("HugoWorkflow.Validate() unexpected error: %v (EpisodeMD=%q)", err, episodeMD)
				}
			})
		}
	})
}

func existingMarkdownArgument(t *testing.T, path string) string {
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
		t.Fatalf("create markdown fixture dir: %v", err)
	}
	if err := os.WriteFile(target, []byte("---\n---\n"), 0o644); err != nil {
		t.Fatalf("create markdown fixture: %v", err)
	}

	return arg
}
