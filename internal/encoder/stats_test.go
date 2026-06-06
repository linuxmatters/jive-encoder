package encoder

import (
	"os"
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

// TestGetFileStats is an integration test that requires an actual MP3 file
// The MP3 is created by TestEncodeToMP3_Integration if it doesn't exist
func TestGetFileStats(t *testing.T) {
	testFile := "../../testdata/LMP0.mp3"

	// Skip if test file doesn't exist
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test MP3 file not found - run tests to generate it via TestEncodeToMP3_Integration")
	}

	// Use a known duration for testing (the test file is ~27 seconds)
	testDurationSecs := int64(27)
	stats, err := GetFileStats(testFile, testDurationSecs)
	if err != nil {
		t.Fatalf("GetFileStats() error = %v", err)
	}

	// Verify duration format (should be HH:MM:SS)
	if len(stats.DurationString) != 8 {
		t.Errorf("Duration format incorrect: got %s, want HH:MM:SS format", stats.DurationString)
	}

	if stats.FileSizeBytes <= 0 {
		t.Errorf("FileSizeBytes = %d; want > 0", stats.FileSizeBytes)
	}

	t.Logf("Stats for %s: duration=%s, size=%d bytes",
		testFile, stats.DurationString, stats.FileSizeBytes)
}
