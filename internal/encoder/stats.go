package encoder

import (
	"fmt"
	"os"
)

// FileStats holds podcast frontmatter statistics
type FileStats struct {
	DurationString string // HH:MM:SS format
	FileSizeBytes  int64  // File size in bytes
}

// GetFileStats returns file statistics using a pre-calculated duration.
// This avoids re-opening the encoded file with FFmpeg to extract duration.
func GetFileStats(outputPath string, durationSecs int64) (*FileStats, error) {
	fileInfo, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	durationStr := formatDurationHMS(durationSecs)

	return &FileStats{
		DurationString: durationStr,
		FileSizeBytes:  fileInfo.Size(),
	}, nil
}

// formatDurationHMS converts seconds to HH:MM:SS format
func formatDurationHMS(seconds int64) string {
	hours := seconds / 3600
	minutes := (seconds / 60) % 60
	secs := seconds % 60

	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}
