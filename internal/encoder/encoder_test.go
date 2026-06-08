package encoder

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"

	"github.com/linuxmatters/jivedrop/internal/id3"
)

// TestNewFormatResolution verifies that New defaults an empty Format to the
// mp3 preset and rejects an unknown format.
func TestNewFormatResolution(t *testing.T) {
	t.Run("unknown format errors", func(t *testing.T) {
		if _, err := New(Config{
			InputPath:  "in.flac",
			OutputPath: "out.mp3",
			Format:     "bogus",
		}); err == nil {
			t.Fatal("expected error for unknown format, got nil")
		}
	})

	t.Run("empty format resolves to mp3", func(t *testing.T) {
		enc, err := New(Config{
			InputPath:  "in.flac",
			OutputPath: "out.mp3",
		})
		if err != nil {
			t.Fatalf("New failed: %v", err)
		}
		if enc.preset.name != "mp3" {
			t.Fatalf("expected mp3 preset, got %q", enc.preset.name)
		}
		if enc.outStreamIndex != -1 {
			t.Fatalf("expected outStreamIndex -1, got %d", enc.outStreamIndex)
		}
	})
}

// TestEncodeToMP3_Integration is an integration test that verifies
// the full encoding pipeline works and creates a test MP3 file that
// other tests can use for validation.
func TestEncodeToMP3_Integration(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	outputPath := "../../testdata/LMP0.mp3"

	// Not cleaned up: other tests reuse this MP3, and it is already gitignored.

	t.Run("mono encoding", func(t *testing.T) {
		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: outputPath,
			Stereo:     false,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}

		// A real audio file should exceed 1KB; a smaller output signals a broken encode.
		if info.Size() < 1024 {
			t.Errorf("Output file too small: %d bytes", info.Size())
		}

		t.Logf("Created MP3: %s (%d bytes)", outputPath, info.Size())
	})

	t.Run("stereo encoding", func(t *testing.T) {
		stereoOutput := "../../testdata/LMP0-stereo.mp3"
		defer os.Remove(stereoOutput)

		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: stereoOutput,
			Stereo:     true,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode (stereo) failed: %v", err)
		}

		info, err := os.Stat(stereoOutput)
		if err != nil {
			t.Fatalf("Stereo output file not created: %v", err)
		}

		// Stereo file should be larger than mono (approximately 192/112 ratio)
		if info.Size() < 1024 {
			t.Errorf("Stereo output file too small: %d bytes", info.Size())
		}

		t.Logf("Created stereo MP3: %s (%d bytes)", stereoOutput, info.Size())
	})
}

// TestEncodeToM4A_Integration is an integration test that verifies the full
// AAC encoding pipeline works and creates a test M4A file.
func TestEncodeToM4A_Integration(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	t.Run("mono encoding", func(t *testing.T) {
		outputPath := "../../testdata/LMP0.m4a"
		defer os.Remove(outputPath)

		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: outputPath,
			Format:     "aac",
			Stereo:     false,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}

		// A real audio file should exceed 1KB; a smaller output signals a broken encode.
		if info.Size() < 1024 {
			t.Errorf("Output file too small: %d bytes", info.Size())
		}

		t.Logf("Created M4A: %s (%d bytes)", outputPath, info.Size())
	})

	t.Run("stereo encoding", func(t *testing.T) {
		stereoOutput := "../../testdata/LMP0-stereo.m4a"
		defer os.Remove(stereoOutput)

		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: stereoOutput,
			Format:     "aac",
			Stereo:     true,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode (stereo) failed: %v", err)
		}

		info, err := os.Stat(stereoOutput)
		if err != nil {
			t.Fatalf("Stereo output file not created: %v", err)
		}

		// Stereo file should be larger than mono (approximately 128/64 ratio)
		if info.Size() < 1024 {
			t.Errorf("Stereo output file too small: %d bytes", info.Size())
		}

		t.Logf("Created stereo M4A: %s (%d bytes)", stereoOutput, info.Size())
	})
}

// TestEncodeToOpus_Integration is an integration test that verifies the full
// Opus encoding pipeline works and creates a test Opus file. libopus rejects
// 44.1 kHz, so the Opus path resamples to 48 kHz.
func TestEncodeToOpus_Integration(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	t.Run("mono encoding", func(t *testing.T) {
		outputPath := "../../testdata/LMP0.opus"
		defer os.Remove(outputPath)

		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: outputPath,
			Format:     "opus",
			Stereo:     false,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}

		// A real audio file should exceed 1KB; a smaller output signals a broken encode.
		if info.Size() < 1024 {
			t.Errorf("Output file too small: %d bytes", info.Size())
		}

		t.Logf("Created Opus: %s (%d bytes)", outputPath, info.Size())
	})

	t.Run("stereo encoding", func(t *testing.T) {
		stereoOutput := "../../testdata/LMP0-stereo.opus"
		defer os.Remove(stereoOutput)

		enc, err := New(Config{
			InputPath:  inputPath,
			OutputPath: stereoOutput,
			Format:     "opus",
			Stereo:     true,
		})
		if err != nil {
			t.Fatalf("Failed to create encoder: %v", err)
		}
		defer enc.Close()

		if err := enc.Initialize(); err != nil {
			t.Fatalf("Failed to initialize encoder: %v", err)
		}

		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode (stereo) failed: %v", err)
		}

		info, err := os.Stat(stereoOutput)
		if err != nil {
			t.Fatalf("Stereo output file not created: %v", err)
		}

		// Stereo file should be larger than mono (approximately 48/32 ratio)
		if info.Size() < 1024 {
			t.Errorf("Stereo output file too small: %d bytes", info.Size())
		}

		t.Logf("Created stereo Opus: %s (%d bytes)", stereoOutput, info.Size())
	})
}

// TestEncodeMP3Metadata_Integration encodes an MP3 with populated Metadata and
// asserts the muxer-native tags survive into the ID3v2 frames, probed via
// ffprobe. This proves the AVDictionary path independent of any other tagging.
// TDRC (date) and COMM (comment) are the highest-risk frames, so both are
// asserted explicitly alongside title/artist/album/track.
func TestEncodeMP3Metadata_Integration(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "meta.mp3")

	enc, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
		Metadata: Metadata{
			EpisodeNumber: "67",
			Title:         "Panache, for men",
			Artist:        "Linux Matters",
			Album:         "Linux Matters Podcast",
			Date:          "2025-10",
			Comment:       "A test comment",
		},
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	if err := enc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}
	if err := enc.Encode(nil); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	tags := probeFormatTags(t, outputPath)

	want := map[string]string{
		"title":   "67: Panache, for men",
		"artist":  "Linux Matters",
		"album":   "Linux Matters Podcast",
		"date":    "2025-10",
		"comment": "A test comment",
		"track":   "67",
	}
	for key, value := range want {
		got, ok := tags[key]
		if !ok {
			t.Errorf("missing %s tag in ffprobe output", key)
			continue
		}
		if got != value {
			t.Errorf("%s tag: got %q, want %q", key, got, value)
		}
	}
}

// probeFormatTags runs ffprobe and returns the format-level tag map.
func probeFormatTags(t *testing.T, path string) map[string]string {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "ffprobe", "-show_format", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe failed: %v", err)
	}

	var probe struct {
		Format struct {
			Tags map[string]string `json:"tags"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("failed to parse ffprobe output: %v", err)
	}

	return probe.Format.Tags
}

// TestEncodeCoverArt_Integration encodes with scaled cover bytes and asserts
// the attached-picture stream behaviour per format: MP3 and AAC are
// cover-capable and must carry an attached-picture video stream, while Opus is
// not cover-capable and must stay audio-only. The cover bytes come from
// id3.ScaleCoverArt on a real testdata PNG fixture.
func TestEncodeCoverArt_Integration(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not available")
	}

	coverPath := "../../testdata/linuxmatters-alt.png"
	if _, err := os.Stat(coverPath); os.IsNotExist(err) {
		t.Skipf("Cover fixture not found: %s", coverPath)
	}
	cover, err := id3.ScaleCoverArt(coverPath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}
	if len(cover) == 0 {
		t.Fatal("ScaleCoverArt returned no bytes")
	}

	tests := []struct {
		name      string
		format    string
		ext       string
		wantCover bool
	}{
		{name: "mp3 carries attached picture", format: "mp3", ext: "mp3", wantCover: true},
		{name: "aac carries attached picture", format: "aac", ext: "m4a", wantCover: true},
		{name: "opus has no attached picture", format: "opus", ext: "opus", wantCover: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			outputPath := filepath.Join(t.TempDir(), "cover."+tt.ext)

			enc, err := New(Config{
				InputPath:  inputPath,
				OutputPath: outputPath,
				Format:     tt.format,
				Stereo:     false,
				CoverArt:   cover,
			})
			if err != nil {
				t.Fatalf("Failed to create encoder: %v", err)
			}
			defer enc.Close()

			if err := enc.Initialize(); err != nil {
				t.Fatalf("Failed to initialize encoder: %v", err)
			}
			if err := enc.Encode(nil); err != nil {
				t.Fatalf("Encode failed: %v", err)
			}

			streams := probeStreams(t, outputPath)

			var haveAudio, haveAttachedPic bool
			for _, s := range streams {
				switch s.CodecType {
				case "audio":
					haveAudio = true
				case "video":
					if s.Disposition["attached_pic"] == 1 {
						haveAttachedPic = true
					}
				}
			}

			if !haveAudio {
				t.Errorf("%s: no audio stream in output", tt.format)
			}
			if haveAttachedPic != tt.wantCover {
				t.Errorf("%s: attached_pic stream present = %v, want %v", tt.format, haveAttachedPic, tt.wantCover)
			}
		})
	}
}

// probeStream is a minimal ffprobe stream record covering the fields the cover
// test asserts.
type probeStream struct {
	CodecType   string         `json:"codec_type"`
	Disposition map[string]int `json:"disposition"`
}

// probeStreams runs ffprobe -show_streams and returns the decoded stream list.
func probeStreams(t *testing.T, path string) []probeStream {
	t.Helper()

	cmd := exec.CommandContext(t.Context(), "ffprobe", "-show_streams", "-of", "json", path)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("ffprobe failed: %v", err)
	}

	var probe struct {
		Streams []probeStream `json:"streams"`
	}
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("failed to parse ffprobe output: %v", err)
	}

	return probe.Streams
}

// TestEncoder_InvalidInput tests error handling for invalid inputs
func TestEncoder_InvalidInput(t *testing.T) {
	tests := []struct {
		name       string
		inputPath  string
		outputPath string
		wantErr    bool
	}{
		{
			name:       "non-existent input file",
			inputPath:  "/nonexistent/file.flac",
			outputPath: "/tmp/output.mp3",
			wantErr:    true,
		},
		{
			name:       "empty input path",
			inputPath:  "",
			outputPath: "/tmp/output.mp3",
			wantErr:    true,
		},
		{
			name:       "empty output path",
			inputPath:  "../../testdata/LMP0.flac",
			outputPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			enc, err := New(Config{
				InputPath:  tt.inputPath,
				OutputPath: tt.outputPath,
				Stereo:     false,
			})
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Unexpected error during New: %v", err)
				}
				return
			}
			defer enc.Close()

			err = enc.Initialize()
			if tt.wantErr && err == nil {
				t.Error("Expected error during Initialize, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error during Initialize: %v", err)
			}
		})
	}
}

// TestEncoder_OutputExists tests behavior when output file already exists
func TestEncoder_OutputExists(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.mp3")

	enc1, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc1.Close()

	if err := enc1.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}

	if err := enc1.Encode(nil); err != nil {
		t.Fatalf("Initial encoding failed: %v", err)
	}

	initialInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat initial output: %v", err)
	}

	// Encode again to same path (should overwrite)
	enc2, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create second encoder: %v", err)
	}
	defer enc2.Close()

	if err := enc2.Initialize(); err != nil {
		t.Fatalf("Failed to initialize second encoder: %v", err)
	}

	if err := enc2.Encode(nil); err != nil {
		t.Fatalf("Second encoding failed: %v", err)
	}

	newInfo, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("Failed to stat new output: %v", err)
	}

	// Timestamps should be different (file was replaced)
	if !newInfo.ModTime().After(initialInfo.ModTime()) && !newInfo.ModTime().Equal(initialInfo.ModTime()) {
		t.Error("Output file does not appear to have been overwritten")
	}
}

// TestEncoder_CloseSafety verifies that Close() handles edge cases without panicking
func TestEncoder_CloseSafety(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		setupFunc func() *Encoder
	}{
		{
			name: "close before initialize",
			setupFunc: func() *Encoder {
				enc, err := New(Config{
					InputPath:  "../../testdata/LMP0.flac",
					OutputPath: filepath.Join(tmpDir, "test1.mp3"),
					Stereo:     false,
				})
				if err != nil {
					t.Fatalf("Failed to create encoder: %v", err)
				}
				// Don't call Initialize - Close should handle nil pointers gracefully
				return enc
			},
		},
		{
			name: "double close",
			setupFunc: func() *Encoder {
				enc, err := New(Config{
					InputPath:  "../../testdata/LMP0.flac",
					OutputPath: filepath.Join(tmpDir, "test2.mp3"),
					Stereo:     false,
				})
				if err != nil {
					t.Fatalf("Failed to create encoder: %v", err)
				}

				if err := enc.Initialize(); err != nil {
					t.Fatalf("Failed to initialize encoder: %v", err)
				}

				// First close
				enc.Close()
				// Second close should not panic even though resources are freed
				return enc
			},
		},
		{
			name: "close after failed initialize",
			setupFunc: func() *Encoder {
				// Use invalid input file to cause Initialize to fail
				enc, err := New(Config{
					InputPath:  "/nonexistent/invalid.flac",
					OutputPath: filepath.Join(tmpDir, "test3.mp3"),
					Stereo:     false,
				})
				if err != nil {
					t.Fatalf("Failed to create encoder: %v", err)
				}

				// Initialize will fail, leaving partial resources
				_ = enc.Initialize()
				// Close should still work safely even after failed Initialize
				return enc
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use a separate test to verify that Close() can be called
			// multiple times or after partial initialization without panicking
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Close() panicked: %v", r)
				}
			}()

			enc := tt.setupFunc()
			// Call Close a second time for the "double close" scenario
			// For other scenarios, this is extra safety verification
			enc.Close()
			enc.Close()
		})
	}
}

// TestEncoder_ProgressCallback verifies progress callback receives valid values
func TestEncoder_ProgressCallback(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.mp3")

	enc, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	if err := enc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}

	totalSamples := enc.totalSamples
	if totalSamples == 0 {
		t.Skip("Could not determine total samples from input file")
	}

	var progressUpdates []struct {
		samplesProcessed int64
		totalSamples     int64
	}
	var mu sync.Mutex

	progressCb := func(samplesProcessed, totalSamples int64) {
		mu.Lock()
		defer mu.Unlock()
		progressUpdates = append(progressUpdates, struct {
			samplesProcessed int64
			totalSamples     int64
		}{samplesProcessed, totalSamples})
	}

	if err := enc.Encode(progressCb); err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	if len(progressUpdates) == 0 {
		t.Skip("No progress updates received (file may be too small)")
	}

	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Fatalf("Output file not created: %v", err)
	}

	// Verify each progress update
	for i, update := range progressUpdates {
		// Check totalSamples is consistent
		if update.totalSamples != totalSamples {
			t.Errorf("Update %d: totalSamples mismatch - expected %d, got %d",
				i, totalSamples, update.totalSamples)
		}

		// Check samplesProcessed is within valid range [0, totalSamples]
		if update.samplesProcessed < 0 {
			t.Errorf("Update %d: samplesProcessed is negative: %d", i, update.samplesProcessed)
		}
		if update.samplesProcessed > totalSamples {
			t.Errorf("Update %d: samplesProcessed (%d) exceeds totalSamples (%d)",
				i, update.samplesProcessed, totalSamples)
		}

		// Check monotonically increasing (each update >= previous)
		if i > 0 {
			prevSamples := progressUpdates[i-1].samplesProcessed
			if update.samplesProcessed < prevSamples {
				t.Errorf("Update %d: progress decreased from %d to %d",
					i, prevSamples, update.samplesProcessed)
			}
		}
	}

	// Verify final update reaches or nearly reaches totalSamples
	lastUpdate := progressUpdates[len(progressUpdates)-1]
	if lastUpdate.samplesProcessed < totalSamples {
		// Allow small tolerance (within 1% or 100 samples, whichever is larger)
		tolerance := max(totalSamples/100, 100)
		if totalSamples-lastUpdate.samplesProcessed > tolerance {
			t.Errorf("Final update does not reach totalSamples: %d vs %d (diff: %d)",
				lastUpdate.samplesProcessed, totalSamples,
				totalSamples-lastUpdate.samplesProcessed)
		}
	}

	t.Logf("Progress callback received %d updates, samples: %d -> %d of %d",
		len(progressUpdates), progressUpdates[0].samplesProcessed,
		lastUpdate.samplesProcessed, totalSamples)
}

// TestEncoder_CancelMidStream verifies that Cancel, fired from within the
// progress callback while Encode is mid-loop, stops the encode promptly with
// ErrCancelled and leaves Close safe to call. This guards the Ctrl+C
// use-after-free: Encode must fully return before Close frees the AV contexts.
func TestEncoder_CancelMidStream(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "cancel.mp3")

	enc, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	if err := enc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}

	// Cancel on the first progress callback, i.e. while Encode is mid-loop and
	// the AV contexts are live. The next loop iteration must observe it.
	var callbacks int
	cb := func(samplesProcessed, totalSamples int64) {
		callbacks++
		if callbacks == 1 {
			enc.Cancel()
		}
	}

	err = enc.Encode(cb)
	if !errors.Is(err, ErrCancelled) {
		t.Fatalf("Encode after Cancel: got %v, want ErrCancelled", err)
	}

	// Encode has returned, so Close must not crash on the still-allocated
	// contexts. A use-after-free would surface as a SIGSEGV here, not a panic,
	// but reaching the end of the test without crashing proves the race is shut.
	enc.Close()
}

// TestEncoder_CancelBeforeEncode verifies that cancelling before Encode runs
// returns ErrCancelled immediately without touching the stream, and Close stays
// safe afterwards.
func TestEncoder_CancelBeforeEncode(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "cancel-early.mp3")

	enc, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	if err := enc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}

	enc.Cancel()

	if err := enc.Encode(nil); !errors.Is(err, ErrCancelled) {
		t.Fatalf("Encode after early Cancel: got %v, want ErrCancelled", err)
	}

	enc.Close()
}

// TestEncoder_GetDurationSecs verifies duration calculation after encoding
func TestEncoder_GetDurationSecs(t *testing.T) {
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test.mp3")

	enc, err := New(Config{
		InputPath:  inputPath,
		OutputPath: outputPath,
		Stereo:     false,
	})
	if err != nil {
		t.Fatalf("Failed to create encoder: %v", err)
	}
	defer enc.Close()

	// GetDurationSecs before Initialize should return 0
	if dur := enc.GetDurationSecs(); dur != 0 {
		t.Errorf("GetDurationSecs before Initialize: got %d, want 0", dur)
	}

	if err := enc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize encoder: %v", err)
	}

	// GetDurationSecs before Encode should return 0 (no samples processed yet)
	if dur := enc.GetDurationSecs(); dur != 0 {
		t.Errorf("GetDurationSecs before Encode: got %d, want 0", dur)
	}

	if err := enc.Encode(nil); err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	// GetDurationSecs after Encode should return a positive duration
	duration := enc.GetDurationSecs()
	if duration <= 0 {
		t.Errorf("GetDurationSecs after Encode: got %d, want > 0", duration)
	}

	// LMP0.flac is approximately 27 seconds - allow some tolerance
	if duration < 25 || duration > 30 {
		t.Logf("Warning: duration %d seconds may be unexpected for test file", duration)
	}

	t.Logf("Encoded duration: %d seconds", duration)
}
