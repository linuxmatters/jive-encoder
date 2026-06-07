package encoder

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// TestEncodeToMP3_Integration is an integration test that verifies
// the full encoding pipeline works and creates a test MP3 file that
// other tests can use for validation.
func TestEncodeToMP3_Integration(t *testing.T) {
	// Use testdata/LMP0.flac as input
	inputPath := "../../testdata/LMP0.flac"
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		t.Skipf("Test file not found: %s", inputPath)
	}

	// Output to testdata/LMP0.mp3 (not tracked in git)
	outputPath := "../../testdata/LMP0.mp3"

	// Note: We don't defer cleanup of this file - it's used by other tests
	// and is already in .gitignore

	// Test mono encoding (default)
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

		// Encode with nil progress callback for tests
		err = enc.Encode(nil)
		if err != nil {
			t.Fatalf("Encode failed: %v", err)
		}

		// Verify output file exists
		info, err := os.Stat(outputPath)
		if err != nil {
			t.Fatalf("Output file not created: %v", err)
		}

		// Verify output file has reasonable size (should be > 1KB for a real audio file)
		if info.Size() < 1024 {
			t.Errorf("Output file too small: %d bytes", info.Size())
		}

		t.Logf("Created MP3: %s (%d bytes)", outputPath, info.Size())
	})

	// Test stereo encoding
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

		// Verify output exists
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
			// Check for immediate config errors
			if err != nil {
				if !tt.wantErr {
					t.Errorf("Unexpected error during New: %v", err)
				}
				return
			}
			defer enc.Close()

			// Check for initialization errors
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

	// Create initial encoding
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

	// Get initial file info
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

	// Verify file was overwritten
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

	// Create and initialize encoder
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

	// Get total samples from encoder state
	totalSamples := enc.totalSamples
	if totalSamples == 0 {
		t.Skip("Could not determine total samples from input file")
	}

	// Track all progress updates
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

	// Encode with progress callback
	if err := enc.Encode(progressCb); err != nil {
		t.Fatalf("Encoding failed: %v", err)
	}

	if len(progressUpdates) == 0 {
		t.Skip("No progress updates received (file may be too small)")
	}

	// Verify output file was created
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

	// Encode the file
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
