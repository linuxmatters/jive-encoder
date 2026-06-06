package id3

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bogem/id3v2/v2"
)

func TestWriteTags(t *testing.T) {
	// Create a minimal valid MP3 file for testing
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3 from testdata
	// The MP3 is created by encoder integration tests if it doesn't exist
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found at %s: %v", testMP3, err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Test tag info
	info := TagInfo{
		EpisodeNumber: "67",
		Title:         "Mirrors, Motors and Makefiles",
		Artist:        "Linux Matters",
		Album:         "Linux Matters",
		Date:          "2025-11",
		Comment:       "https://linuxmatters.sh/",
	}

	// Write tags
	err = WriteTags(mp3Path, info)
	if err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}

	// Verify tags were written correctly
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open MP3 for verification: %v", err)
	}
	defer tag.Close()

	// Verify TIT2 (Title)
	expectedTitle := "67: Mirrors, Motors and Makefiles"
	if tag.Title() != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, tag.Title())
	}

	// Verify TALB (Album)
	if tag.Album() != "Linux Matters" {
		t.Errorf("Expected album 'Linux Matters', got '%s'", tag.Album())
	}

	// Verify TPE1 (Artist)
	if tag.Artist() != "Linux Matters" {
		t.Errorf("Expected artist 'Linux Matters', got '%s'", tag.Artist())
	}

	// Verify COMM (Comment)
	comments := tag.GetFrames(tag.CommonID("Comments"))
	if len(comments) == 0 {
		t.Error("Expected comment frame, got none")
	} else {
		commentFrame, ok := comments[0].(id3v2.CommentFrame)
		if !ok {
			t.Error("Comment frame is not of type CommentFrame")
		} else if commentFrame.Text != "https://linuxmatters.sh/" {
			t.Errorf("Expected comment 'https://linuxmatters.sh/', got '%s'", commentFrame.Text)
		}
	}
}

func TestWriteTags_NonExistentFile(t *testing.T) {
	info := TagInfo{
		EpisodeNumber: "67",
		Title:         "Test Episode",
		Date:          "2025-11",
	}

	err := WriteTags("/nonexistent/path/test.mp3", info)
	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
}

func TestWriteTags_WithDate(t *testing.T) {
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found: %v", err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	info := TagInfo{
		EpisodeNumber: "67",
		Title:         "Test Episode",
		Date:          "2025-11",
	}

	if err := WriteTags(mp3Path, info); err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}

	// Verify date tag
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open MP3: %v", err)
	}
	defer tag.Close()

	dateFrames := tag.GetFrames(tag.CommonID("Recording time"))
	if len(dateFrames) == 0 {
		t.Error("Expected recording time frame, got none")
	}
}

// TestWriteTags_WithCoverArt tests ID3 tag writing with cover artwork embedded
func TestWriteTags_WithCoverArt(t *testing.T) {
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3 from testdata
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found at %s: %v", testMP3, err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Use the real test image from testdata
	coverArtPath := "../../testdata/linuxmatters-3000x3000.png"
	if _, err := os.Stat(coverArtPath); err != nil {
		t.Skipf("Test cover art not found at %s", coverArtPath)
	}

	// Pre-process cover art
	coverArtData, err := ScaleCoverArt(coverArtPath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// Create tag info with cover art
	info := TagInfo{
		EpisodeNumber: "67",
		Title:         "Mirrors, Motors and Makefiles",
		Artist:        "Linux Matters",
		Album:         "Linux Matters",
		Date:          "2025-11",
		Comment:       "https://linuxmatters.sh/",
		CoverArtData:  coverArtData,
	}

	// Write tags with cover art
	err = WriteTags(mp3Path, info)
	if err != nil {
		t.Fatalf("WriteTags with cover art failed: %v", err)
	}

	// Verify tags and cover art were written
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open MP3 for verification: %v", err)
	}
	defer tag.Close()

	// Verify basic tags still work
	expectedTitle := "67: Mirrors, Motors and Makefiles"
	if tag.Title() != expectedTitle {
		t.Errorf("Expected title '%s', got '%s'", expectedTitle, tag.Title())
	}

	// Verify APIC frame (cover art) exists
	pictures := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pictures) == 0 {
		t.Error("Expected APIC frame for cover art, got none")
	} else {
		pic, ok := pictures[0].(id3v2.PictureFrame)
		if !ok {
			t.Error("Picture frame is not of type PictureFrame")
		} else {
			// Verify picture data is not empty
			if len(pic.Picture) == 0 {
				t.Error("Picture frame data is empty")
			}

			// Verify picture type is front cover
			if pic.PictureType != id3v2.PTFrontCover {
				t.Errorf("Expected picture type PTFrontCover, got %d", pic.PictureType)
			}

			// Verify MIME type is PNG
			if pic.MimeType != "image/png" {
				t.Errorf("Expected MIME type 'image/png', got '%s'", pic.MimeType)
			}

			// Verify picture data is reasonable size (should be several KB for a 3000x3000 image)
			if len(pic.Picture) < 1024 {
				t.Errorf("Picture data too small: %d bytes (expected > 1KB)", len(pic.Picture))
			}
		}
	}
}

// TestWriteTags_WithCoverArt_InvalidPath tests error handling for missing cover art
func TestWriteTags_WithCoverArt_InvalidPath(t *testing.T) {
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3 from testdata
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found at %s: %v", testMP3, err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Attempt to pre-process non-existent cover art
	_, err = ScaleCoverArt("/nonexistent/path/to/cover.png")
	if err == nil {
		t.Error("Expected error for non-existent cover art, got nil")
	}
}

// TestWriteTags_WithCoverArt_AllMetadata tests WriteTags with all fields and cover art
func TestWriteTags_WithCoverArt_AllMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3 from testdata
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found: %v", err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Get cover art path
	coverArtPath := "../../testdata/linuxmatters-3000x3000.png"
	if _, err := os.Stat(coverArtPath); err != nil {
		t.Skipf("Test cover art not found")
	}

	// Pre-process cover art
	coverArtData, err := ScaleCoverArt(coverArtPath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// Create comprehensive tag info
	info := TagInfo{
		EpisodeNumber: "42",
		Title:         "The Quest Begins",
		Artist:        "Adventure Podcast",
		Album:         "Season 2",
		Date:          "2025-12",
		Comment:       "https://adventurepodcast.example.com/episode-42",
		CoverArtData:  coverArtData,
	}

	// Write all tags
	err = WriteTags(mp3Path, info)
	if err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}

	// Verify all tags including cover art
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open MP3: %v", err)
	}
	defer tag.Close()

	// Verify TIT2
	expectedTitle := "42: The Quest Begins"
	if tag.Title() != expectedTitle {
		t.Errorf("Title mismatch: got %q, want %q", tag.Title(), expectedTitle)
	}

	// Verify TALB
	if tag.Album() != "Season 2" {
		t.Errorf("Album mismatch: got %q, want %q", tag.Album(), "Season 2")
	}

	// Verify TPE1
	if tag.Artist() != "Adventure Podcast" {
		t.Errorf("Artist mismatch: got %q, want %q", tag.Artist(), "Adventure Podcast")
	}

	// Verify TRCK
	frames := tag.GetFrames(tag.CommonID("Track number/Position in set"))
	if len(frames) == 0 {
		t.Error("Expected track number frame, got none")
	}

	// Verify TDRC
	dateFrames := tag.GetFrames(tag.CommonID("Recording time"))
	if len(dateFrames) == 0 {
		t.Error("Expected recording time frame, got none")
	}

	// Verify COMM
	comments := tag.GetFrames(tag.CommonID("Comments"))
	if len(comments) == 0 {
		t.Error("Expected comment frame, got none")
	} else {
		comment, ok := comments[0].(id3v2.CommentFrame)
		if !ok {
			t.Error("Comment frame is wrong type")
		} else if comment.Text != "https://adventurepodcast.example.com/episode-42" {
			t.Errorf("Comment mismatch: got %q", comment.Text)
		}
	}

	// Verify APIC (cover art)
	pictures := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pictures) == 0 {
		t.Fatal("Expected cover art frame, got none")
	}

	pic, ok := pictures[0].(id3v2.PictureFrame)
	if !ok {
		t.Fatal("Picture frame is wrong type")
	}

	if len(pic.Picture) == 0 {
		t.Error("Picture data is empty")
	}

	if pic.PictureType != id3v2.PTFrontCover {
		t.Errorf("Wrong picture type: got %d, want %d", pic.PictureType, id3v2.PTFrontCover)
	}

	if pic.MimeType != "image/png" {
		t.Errorf("Wrong MIME type: got %q, want %q", pic.MimeType, "image/png")
	}
}

// TestWriteTags_CoverArt_NoOtherMetadata tests cover art with minimal metadata
func TestWriteTags_CoverArt_NoOtherMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	mp3Path := filepath.Join(tmpDir, "test.mp3")

	// Copy test MP3
	testMP3 := "../../testdata/LMP0.mp3"
	input, err := os.ReadFile(testMP3)
	if err != nil {
		t.Skipf("Test MP3 not found: %v", err)
	}
	if err := os.WriteFile(mp3Path, input, 0o644); err != nil { //nolint:gosec // test file path from t.TempDir
		t.Fatalf("Failed to create test MP3: %v", err)
	}

	// Get cover art
	coverArtPath := "../../testdata/linuxmatters-3000x3000.png"
	if _, err := os.Stat(coverArtPath); err != nil {
		t.Skipf("Test cover art not found")
	}

	// Pre-process cover art
	coverArtData, err := ScaleCoverArt(coverArtPath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// Minimal tag info with only required fields and cover art
	info := TagInfo{
		EpisodeNumber: "1",
		Title:         "Pilot",
		CoverArtData:  coverArtData,
	}

	// Write tags
	err = WriteTags(mp3Path, info)
	if err != nil {
		t.Fatalf("WriteTags failed: %v", err)
	}

	// Verify minimal tags and cover art
	tag, err := id3v2.Open(mp3Path, id3v2.Options{Parse: true})
	if err != nil {
		t.Fatalf("Failed to open MP3: %v", err)
	}
	defer tag.Close()

	// Verify title with episode number
	if tag.Title() != "1: Pilot" {
		t.Errorf("Title: got %q, want %q", tag.Title(), "1: Pilot")
	}

	// Verify cover art was added
	pictures := tag.GetFrames(tag.CommonID("Attached picture"))
	if len(pictures) == 0 {
		t.Error("Cover art not found")
	} else {
		pic, ok := pictures[0].(id3v2.PictureFrame)
		if !ok || len(pic.Picture) == 0 {
			t.Error("Cover art data invalid")
		}
	}
}
