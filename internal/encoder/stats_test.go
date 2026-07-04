package encoder

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFormatDurationHMS(t *testing.T) {
	tests := []struct {
		seconds  int64
		expected string
	}{
		{27, "00:00:27"},    // Under a minute
		{600, "00:10:00"},   // Exactly 10 minutes
		{1695, "00:28:15"},  // 28 minutes 15 seconds
		{3661, "01:01:01"},  // Over an hour
		{36000, "10:00:00"}, // Exactly 10 hours
		{86399, "23:59:59"}, // Max in a day
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := formatDurationHMS(tt.seconds)
			if result != tt.expected {
				t.Errorf("formatDurationHMS(%d) = %s; want %s", tt.seconds, result, tt.expected)
			}
		})
	}
}

// TestGetFileStats verifies size and duration reporting against temp files
// of known content, so it needs no encoded audio artefacts.
func TestGetFileStats(t *testing.T) {
	tests := []struct {
		name         string
		sizeBytes    int
		durationSecs int64
		expected     string
	}{
		{"empty file, zero duration", 0, 0, "00:00:00"},
		{"small file, under a minute", 27, 27, "00:00:27"},
		{"typical episode length", 4096, 1695, "00:28:15"},
		{"over an hour", 1024, 3661, "01:01:01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "episode.mp3")
			if err := os.WriteFile(path, make([]byte, tt.sizeBytes), 0o644); err != nil {
				t.Fatalf("failed to write temp file: %v", err)
			}

			stats, err := GetFileStats(path, tt.durationSecs)
			if err != nil {
				t.Fatalf("GetFileStats() error = %v", err)
			}

			if stats.FileSizeBytes != int64(tt.sizeBytes) {
				t.Errorf("FileSizeBytes = %d; want %d", stats.FileSizeBytes, tt.sizeBytes)
			}
			if stats.DurationString != tt.expected {
				t.Errorf("DurationString = %s; want %s", stats.DurationString, tt.expected)
			}
		})
	}

	t.Run("missing file returns error", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "missing.mp3")
		if _, err := GetFileStats(missing, 27); err == nil {
			t.Error("GetFileStats() on missing file: expected error, got nil")
		}
	})
}
