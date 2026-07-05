package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/linuxmatters/jive-encoder/internal/encoder"
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

// TestHugoWorkflowCollectMetadata covers flag-over-frontmatter precedence,
// episode validation, date formatting, and cover-path resolution.
func TestHugoWorkflowCollectMetadata(t *testing.T) {
	t.Run("frontmatter values with Hugo defaults", func(t *testing.T) {
		mdPath := writeHugoFixture(t, "episode: \"67\"\ntitle: The Show\nDate: 2024-03-15\nepisode_image: ./cover.png\n")
		if err := os.WriteFile(filepath.Join(filepath.Dir(mdPath), "cover.png"), []byte("img"), 0o644); err != nil {
			t.Fatalf("write cover fixture: %v", err)
		}

		wf := &HugoWorkflow{opts: CLIOptions{EpisodeMD: mdPath}}
		meta, cover, err := wf.CollectMetadata()
		if err != nil {
			t.Fatalf("CollectMetadata() unexpected error: %v", err)
		}

		if meta.EpisodeNumber != "67" {
			t.Errorf("EpisodeNumber = %q; want 67", meta.EpisodeNumber)
		}
		if meta.Title != "The Show" {
			t.Errorf("Title = %q; want The Show", meta.Title)
		}
		if meta.Artist != HugoDefaultArtist {
			t.Errorf("Artist = %q; want %q", meta.Artist, HugoDefaultArtist)
		}
		if meta.Album != HugoDefaultArtist {
			t.Errorf("Album = %q; want %q (falls back to artist)", meta.Album, HugoDefaultArtist)
		}
		if meta.Comment != HugoDefaultComment {
			t.Errorf("Comment = %q; want %q", meta.Comment, HugoDefaultComment)
		}
		if meta.Date != "2024-03" {
			t.Errorf("Date = %q; want 2024-03 (formatted via FormatDateForID3)", meta.Date)
		}
		if filepath.Base(cover) != "cover.png" {
			t.Errorf("cover = %q; want a resolved cover.png path", cover)
		}
	})

	t.Run("flags override frontmatter and defaults", func(t *testing.T) {
		mdPath := writeHugoFixture(t, "episode: \"67\"\ntitle: The Show\nDate: 2024-03-15\nepisode_image: ./cover.png\n")
		flagCover := filepath.Join(t.TempDir(), "flag-cover.png")

		wf := &HugoWorkflow{opts: CLIOptions{
			EpisodeMD: mdPath,
			Num:       "99",
			Title:     "Override Title",
			Artist:    "Override Artist",
			Album:     "Override Album",
			Comment:   "https://override.example",
			Date:      "2020-01-02",
			Cover:     flagCover,
		}}
		meta, cover, err := wf.CollectMetadata()
		if err != nil {
			t.Fatalf("CollectMetadata() unexpected error: %v", err)
		}

		if meta.EpisodeNumber != "99" {
			t.Errorf("EpisodeNumber = %q; want 99", meta.EpisodeNumber)
		}
		if meta.Title != "Override Title" {
			t.Errorf("Title = %q; want Override Title", meta.Title)
		}
		if meta.Artist != "Override Artist" {
			t.Errorf("Artist = %q; want Override Artist", meta.Artist)
		}
		if meta.Album != "Override Album" {
			t.Errorf("Album = %q; want Override Album", meta.Album)
		}
		if meta.Comment != "https://override.example" {
			t.Errorf("Comment = %q; want https://override.example", meta.Comment)
		}
		if meta.Date != "2020-01-02" {
			t.Errorf("Date = %q; want raw --date 2020-01-02", meta.Date)
		}
		if cover != flagCover {
			t.Errorf("cover = %q; want flag cover %q", cover, flagCover)
		}
	})

	t.Run("rejects invalid episode override", func(t *testing.T) {
		mdPath := writeHugoFixture(t, "episode: \"67\"\ntitle: The Show\nepisode_image: ./cover.png\n")
		wf := &HugoWorkflow{opts: CLIOptions{EpisodeMD: mdPath, Num: "-5"}}

		_, _, err := wf.CollectMetadata()
		if err == nil {
			t.Fatal("CollectMetadata() expected error for invalid episode number, got nil")
		}
		if !strings.Contains(err.Error(), "invalid episode number") {
			t.Errorf("error %q does not contain %q", err.Error(), "invalid episode number")
		}
	})

	t.Run("reports cover resolution failure", func(t *testing.T) {
		mdPath := writeHugoFixture(t, "episode: \"67\"\ntitle: The Show\nepisode_image: ./missing.png\n")
		wf := &HugoWorkflow{opts: CLIOptions{EpisodeMD: mdPath}}

		_, _, err := wf.CollectMetadata()
		if err == nil {
			t.Fatal("CollectMetadata() expected error for missing cover art, got nil")
		}
		if !strings.Contains(err.Error(), "resolve cover art") {
			t.Errorf("error %q does not contain %q", err.Error(), "resolve cover art")
		}
	})
}

// TestHugoWorkflowPostEncode covers the mismatch, missing-value, and no-change
// branches that decide whether the user is prompted to update frontmatter.
func TestHugoWorkflowPostEncode(t *testing.T) {
	stats := &encoder.FileStats{DurationString: "00:20:00", FileSizeBytes: 2048}

	tests := []struct {
		name       string
		duration   string
		bytes      int64
		input      string
		wantPrompt string // substring expected in the prompt; "" means no prompt
	}{
		{
			name:       "no change when values match",
			duration:   "00:20:00",
			bytes:      2048,
			input:      "",
			wantPrompt: "",
		},
		{
			name:       "duration mismatch prompts for update",
			duration:   "00:10:00",
			bytes:      2048,
			input:      "n\n",
			wantPrompt: "Update frontmatter with new values?",
		},
		{
			name:       "byte size mismatch prompts for update",
			duration:   "00:20:00",
			bytes:      1024,
			input:      "n\n",
			wantPrompt: "Update frontmatter with new values?",
		},
		{
			name:       "missing values prompt to add",
			duration:   "",
			bytes:      0,
			input:      "n\n",
			wantPrompt: "Add podcast_duration and podcast_bytes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			wf := &HugoWorkflow{
				opts:         CLIOptions{EpisodeMD: filepath.Join(t.TempDir(), "episode.md")},
				hugoMetadata: &encoder.EpisodeMetadata{PodcastDuration: tt.duration, PodcastBytes: tt.bytes},
			}

			var err error
			out := captureStdio(t, tt.input, func() { err = wf.PostEncode(stats) })
			if err != nil {
				t.Fatalf("PostEncode() unexpected error: %v", err)
			}

			if tt.wantPrompt == "" {
				if strings.Contains(out, "frontmatter") {
					t.Errorf("PostEncode() unexpectedly prompted: %q", out)
				}
				return
			}
			if !strings.Contains(out, tt.wantPrompt) {
				t.Errorf("PostEncode() prompt %q does not contain %q", out, tt.wantPrompt)
			}
		})
	}
}

// TestHugoWorkflowPostEncodeWritesFrontmatter verifies the accept path writes
// the calculated stats back into the markdown file.
func TestHugoWorkflowPostEncodeWritesFrontmatter(t *testing.T) {
	mdPath := writeHugoFixture(t, "episode: \"67\"\ntitle: The Show\nepisode_image: ./cover.png\n")
	wf := &HugoWorkflow{
		opts:         CLIOptions{EpisodeMD: mdPath},
		hugoMetadata: &encoder.EpisodeMetadata{},
	}
	stats := &encoder.FileStats{DurationString: "00:20:00", FileSizeBytes: 2048}

	var err error
	captureStdio(t, "y\n", func() { err = wf.PostEncode(stats) })
	if err != nil {
		t.Fatalf("PostEncode() unexpected error: %v", err)
	}

	updated, readErr := os.ReadFile(mdPath)
	if readErr != nil {
		t.Fatalf("read updated markdown: %v", readErr)
	}
	if !strings.Contains(string(updated), "podcast_duration") {
		t.Errorf("frontmatter not updated:\n%s", updated)
	}
}

// TestHugoWorkflowPostEncodePropagatesWriteFailure verifies a failed frontmatter
// write returns an error so the process exits non-zero.
func TestHugoWorkflowPostEncodePropagatesWriteFailure(t *testing.T) {
	wf := &HugoWorkflow{
		opts:         CLIOptions{EpisodeMD: filepath.Join(t.TempDir(), "missing.md")},
		hugoMetadata: &encoder.EpisodeMetadata{},
	}
	stats := &encoder.FileStats{DurationString: "00:20:00", FileSizeBytes: 2048}

	var err error
	captureStdio(t, "y\n", func() { err = wf.PostEncode(stats) })
	if err == nil {
		t.Fatal("PostEncode() expected error when the frontmatter write fails, got nil")
	}
}

// writeHugoFixture writes a markdown file with the given frontmatter body
// (between --- delimiters) and returns its path.
func writeHugoFixture(t *testing.T, frontmatter string) string {
	t.Helper()

	mdPath := filepath.Join(t.TempDir(), "episode.md")
	body := "---\n" + frontmatter + "---\nBody\n"
	if err := os.WriteFile(mdPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write markdown fixture: %v", err)
	}

	return mdPath
}

// captureStdio feeds input to the prompt's stdin and returns whatever fn writes
// to os.Stdout. The cli.Print* helpers hold the original stdout captured at
// package init, so only the raw prompt text is captured here.
func captureStdio(t *testing.T, input string, fn func()) string {
	t.Helper()

	inR, inW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdin pipe: %v", err)
	}
	origStdin := os.Stdin
	os.Stdin = inR
	go func() {
		_, _ = inW.WriteString(input)
		_ = inW.Close()
	}()

	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = outW

	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, outR)
		done <- buf.String()
	}()

	fn()

	os.Stdout = origStdout
	_ = outW.Close()
	captured := <-done

	os.Stdin = origStdin
	_ = inR.Close()

	return captured
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
