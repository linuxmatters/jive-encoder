package artwork

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// pngMagic is the eight-byte PNG file signature.
var pngMagic = []byte("\x89PNG\r\n\x1a\n")

// TestScaleCoverArt_ValidSquareImage tests scaling of valid square images
func TestScaleCoverArt_ValidSquareImage(t *testing.T) {
	tests := []struct {
		name        string
		size        int
		expectSize  int
		shouldScale bool
	}{
		{
			name:        "upscale small image",
			size:        1000,
			expectSize:  1400,
			shouldScale: true,
		},
		{
			name:        "no scale at lower bound",
			size:        1400,
			expectSize:  1400,
			shouldScale: false,
		},
		{
			name:        "no scale in middle range",
			size:        2000,
			expectSize:  2000,
			shouldScale: false,
		},
		{
			name:        "no scale at upper bound",
			size:        3000,
			expectSize:  3000,
			shouldScale: false,
		},
		{
			name:        "downscale large image",
			size:        4000,
			expectSize:  3000,
			shouldScale: true,
		},
		{
			name:        "downscale very large image",
			size:        5000,
			expectSize:  3000,
			shouldScale: true,
		},
		{
			name:        "downscale extreme image",
			size:        10000,
			expectSize:  3000,
			shouldScale: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testImagePath := filepath.Join(tmpDir, "test.png")

			if err := createTestPNG(testImagePath, tt.size, tt.size); err != nil {
				t.Fatalf("Failed to create test PNG: %v", err)
			}

			scaledData, err := ScaleCoverArt(testImagePath)
			if err != nil {
				t.Fatalf("ScaleCoverArt failed: %v", err)
			}

			if scaledData == nil {
				t.Fatal("ScaleCoverArt returned nil data")
			}

			// Verify the scaled image is valid PNG and correct dimensions
			decodedImg, err := png.Decode(bytes.NewReader(scaledData))
			if err != nil {
				t.Fatalf("Failed to decode scaled image: %v", err)
			}

			bounds := decodedImg.Bounds()
			actualSize := bounds.Dx()

			if actualSize != tt.expectSize {
				t.Errorf("Expected scaled size %dx%d, got %dx%d", tt.expectSize, tt.expectSize, actualSize, actualSize)
			}
		})
	}
}

// TestScaleCoverArt_NonSquareImage tests error handling for non-square images
func TestScaleCoverArt_NonSquareImage(t *testing.T) {
	tests := []struct {
		name     string
		width    int
		height   int
		wantErr  bool
		errMatch string
	}{
		{
			name:     "landscape image",
			width:    2000,
			height:   1500,
			wantErr:  true,
			errMatch: "must be square",
		},
		{
			name:     "portrait image",
			width:    1500,
			height:   2000,
			wantErr:  true,
			errMatch: "must be square",
		},
		{
			name:     "wide rectangle",
			width:    3000,
			height:   2000,
			wantErr:  true,
			errMatch: "must be square",
		},
		{
			name:     "tall rectangle",
			width:    1000,
			height:   3000,
			wantErr:  true,
			errMatch: "must be square",
		},
		{
			name:     "almost square",
			width:    1000,
			height:   999,
			wantErr:  true,
			errMatch: "must be square",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testImagePath := filepath.Join(tmpDir, "test.png")

			if err := createTestPNG(testImagePath, tt.width, tt.height); err != nil {
				t.Fatalf("Failed to create test PNG: %v", err)
			}

			_, err := ScaleCoverArt(testImagePath)

			if !tt.wantErr && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if !strings.Contains(err.Error(), tt.errMatch) {
					t.Errorf("Expected error containing '%s', got '%v'", tt.errMatch, err)
				}
			}
		})
	}
}

// TestScaleCoverArt_NonExistentFile tests error handling for missing files
func TestScaleCoverArt_NonExistentFile(t *testing.T) {
	_, err := ScaleCoverArt("/nonexistent/path/to/image.png")

	if err == nil {
		t.Error("Expected error for non-existent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("Expected 'failed to read' in error, got: %v", err)
	}
}

// TestScaleCoverArt_CorruptFile tests error handling for corrupt/invalid image files
func TestScaleCoverArt_CorruptFile(t *testing.T) {
	tmpDir := t.TempDir()
	corruptPath := filepath.Join(tmpDir, "corrupt.png")

	corruptData := []byte{0x89, 0x50, 0x4E, 0x47} // PNG magic but incomplete
	if err := os.WriteFile(corruptPath, corruptData, 0o644); err != nil {
		t.Fatalf("Failed to create corrupt file: %v", err)
	}

	_, err := ScaleCoverArt(corruptPath)

	if err == nil {
		t.Error("Expected error for corrupt file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("Expected 'failed to decode' in error, got: %v", err)
	}
}

// TestScaleCoverArt_TextFile tests error handling for non-image files
func TestScaleCoverArt_TextFile(t *testing.T) {
	tmpDir := t.TempDir()
	textPath := filepath.Join(tmpDir, "notanimage.txt")

	if err := os.WriteFile(textPath, []byte("This is not an image"), 0o644); err != nil {
		t.Fatalf("Failed to create text file: %v", err)
	}

	_, err := ScaleCoverArt(textPath)

	if err == nil {
		t.Error("Expected error for text file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("Expected 'failed to decode' in error, got: %v", err)
	}
}

// TestScaleCoverArt_RealImageFile tests with actual test data
func TestScaleCoverArt_RealImageFile(t *testing.T) {
	testImagePath := "../../testdata/linuxmatters-3000x3000.png"

	if _, err := os.Stat(testImagePath); os.IsNotExist(err) {
		t.Skipf("Test image not found at %s", testImagePath)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	if scaledData == nil {
		t.Fatal("ScaleCoverArt returned nil data")
	}

	// Verify output is valid PNG
	decodedImg, err := png.Decode(bytes.NewReader(scaledData))
	if err != nil {
		t.Fatalf("Failed to decode scaled image: %v", err)
	}

	bounds := decodedImg.Bounds()
	size := bounds.Dx()
	height := bounds.Dy()

	// 3000x3000 should be used as-is (no scaling)
	if size != 3000 || height != 3000 {
		t.Errorf("Expected 3000x3000, got %dx%d", size, height)
	}

	// Verify it's a PNG
	_, format, err := image.Decode(bytes.NewReader(scaledData))
	if err != nil {
		t.Errorf("Failed to verify PNG format: %v", err)
	}
	if format != "png" {
		t.Errorf("Expected PNG format, got %s", format)
	}
}

// TestScaleCoverArt_OutputIsPNG tests that output is always PNG regardless of input
func TestScaleCoverArt_OutputIsPNG(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test.png")

	if err := createTestPNG(testImagePath, 1000, 1000); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// Verify output can be decoded as PNG
	decodedImg, format, err := image.Decode(bytes.NewReader(scaledData))
	if err != nil {
		t.Fatalf("Failed to decode output image: %v", err)
	}
	if decodedImg == nil {
		t.Fatal("Failed to decode output image")
	}
	if format != "png" {
		t.Errorf("Expected PNG output format, got %s", format)
	}
}

// TestScaleCoverArt_SkipPNGReencoding tests that an in-spec PNG passes through
// with its original bytes untouched, avoiding a needless re-encode
func TestScaleCoverArt_SkipPNGReencoding(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test_3000.png")

	// Create a 3000x3000 PNG (already in acceptable range, no scaling needed)
	if err := createTestPNG(testImagePath, 3000, 3000); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	originalData, err := os.ReadFile(testImagePath)
	if err != nil {
		t.Fatalf("Failed to read original file: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	if scaledData == nil {
		t.Fatal("ScaleCoverArt returned nil data")
	}

	// An in-spec PNG must return the original bytes byte-for-byte
	if !bytes.Equal(scaledData, originalData) {
		t.Errorf("Expected original PNG bytes to pass through untouched: got %d bytes, original %d bytes", len(scaledData), len(originalData))
	}
}

// TestScaleCoverArt_InSpecJPEG tests that an in-spec JPEG re-encodes to PNG
// even though no scaling is needed
func TestScaleCoverArt_InSpecJPEG(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test.jpg")

	if err := createTestJPEG(testImagePath, 1500, 1500); err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// The non-PNG path must re-encode to PNG
	if !bytes.HasPrefix(scaledData, pngMagic) {
		t.Errorf("Expected output to start with PNG magic header, got % x", scaledData[:min(len(scaledData), 8)])
	}
}

// TestScaleCoverArt_OutOfSpecJPEG tests that an undersized JPEG scales up
// and re-encodes to PNG
func TestScaleCoverArt_OutOfSpecJPEG(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test.jpg")

	if err := createTestJPEG(testImagePath, 800, 800); err != nil {
		t.Fatalf("Failed to create test JPEG: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	if !bytes.HasPrefix(scaledData, pngMagic) {
		t.Errorf("Expected output to start with PNG magic header, got % x", scaledData[:min(len(scaledData), 8)])
	}

	decodedImg, err := png.Decode(bytes.NewReader(scaledData))
	if err != nil {
		t.Fatalf("Failed to decode scaled image: %v", err)
	}

	bounds := decodedImg.Bounds()
	if bounds.Dx() != 1400 || bounds.Dy() != 1400 {
		t.Errorf("Expected 1400x1400, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// TestScaleCoverArt_OutOfSpecGIF tests that an undersized GIF decodes, scales up
// and re-encodes to PNG
func TestScaleCoverArt_OutOfSpecGIF(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test.gif")

	if err := createTestGIF(testImagePath, 800, 800); err != nil {
		t.Fatalf("Failed to create test GIF: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	if !bytes.HasPrefix(scaledData, pngMagic) {
		t.Errorf("Expected output to start with PNG magic header, got % x", scaledData[:min(len(scaledData), 8)])
	}

	decodedImg, err := png.Decode(bytes.NewReader(scaledData))
	if err != nil {
		t.Fatalf("Failed to decode scaled image: %v", err)
	}

	bounds := decodedImg.Bounds()
	if bounds.Dx() != 1400 || bounds.Dy() != 1400 {
		t.Errorf("Expected 1400x1400, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

// TestScaleCoverArt_EdgeCases tests edge case sizes
func TestScaleCoverArt_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		size       int
		expectSize int
	}{
		{
			name:       "minimum valid size",
			size:       1,
			expectSize: 1400,
		},
		{
			name:       "boundary 1399",
			size:       1399,
			expectSize: 1400,
		},
		{
			name:       "boundary 1401",
			size:       1401,
			expectSize: 1401,
		},
		{
			name:       "boundary 2999",
			size:       2999,
			expectSize: 2999,
		},
		{
			name:       "boundary 3001",
			size:       3001,
			expectSize: 3000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			testImagePath := filepath.Join(tmpDir, "test.png")

			if err := createTestPNG(testImagePath, tt.size, tt.size); err != nil {
				t.Fatalf("Failed to create test PNG: %v", err)
			}

			scaledData, err := ScaleCoverArt(testImagePath)
			if err != nil {
				t.Fatalf("ScaleCoverArt failed: %v", err)
			}

			decodedImg, err := png.Decode(bytes.NewReader(scaledData))
			if err != nil {
				t.Fatalf("Failed to decode scaled image: %v", err)
			}

			bounds := decodedImg.Bounds()
			actualSize := bounds.Dx()

			if actualSize != tt.expectSize {
				t.Errorf("Expected %d, got %d", tt.expectSize, actualSize)
			}
		})
	}
}

// TestScaleCoverArt_OutputDataSize tests that output data is reasonable
func TestScaleCoverArt_OutputDataSize(t *testing.T) {
	tmpDir := t.TempDir()
	testImagePath := filepath.Join(tmpDir, "test.png")

	// Create a small image that will be upscaled
	if err := createTestPNG(testImagePath, 500, 500); err != nil {
		t.Fatalf("Failed to create test PNG: %v", err)
	}

	scaledData, err := ScaleCoverArt(testImagePath)
	if err != nil {
		t.Fatalf("ScaleCoverArt failed: %v", err)
	}

	// Verify output data is not empty and has reasonable size
	// A 1400x1400 PNG should be at least a few KB
	if len(scaledData) < 1024 {
		t.Errorf("Output data too small: %d bytes (expected > 1KB)", len(scaledData))
	}

	// A PNG of this content cannot exceed its uncompressed RGBA pixel data, so
	// 1400x1400x4 bytes is a tight upper bound that still catches a gross regression.
	maxSize := 1400 * 1400 * 4
	if len(scaledData) > maxSize {
		t.Errorf("Output data too large: %d bytes (expected <= %d)", len(scaledData), maxSize)
	}
}

// TestScaleCoverArt_MultipleScalings tests that function works correctly for multiple calls
func TestScaleCoverArt_MultipleScalings(t *testing.T) {
	tmpDir := t.TempDir()

	sizes := []struct {
		inputSize  int
		expectSize int
	}{
		{500, 1400},
		{1400, 1400},
		{2000, 2000},
		{3000, 3000},
		{5000, 3000},
	}

	for i, tt := range sizes {
		testImagePath := filepath.Join(tmpDir, "test_"+strconv.Itoa(i)+".png")

		if err := createTestPNG(testImagePath, tt.inputSize, tt.inputSize); err != nil {
			t.Fatalf("Failed to create test PNG: %v", err)
		}

		scaledData, err := ScaleCoverArt(testImagePath)
		if err != nil {
			t.Fatalf("ScaleCoverArt failed for size %d: %v", tt.inputSize, err)
		}

		decodedImg, err := png.Decode(bytes.NewReader(scaledData))
		if err != nil {
			t.Fatalf("Failed to decode scaled image: %v", err)
		}

		bounds := decodedImg.Bounds()
		actualSize := bounds.Dx()

		if actualSize != tt.expectSize {
			t.Errorf("Input size %d: expected %d, got %d", tt.inputSize, tt.expectSize, actualSize)
		}
	}
}

// Helper function to create test PNG images
func createTestPNG(path string, width, height int) error {
	return createTestImage(path, width, height, png.Encode)
}

// Helper function to create test JPEG images
func createTestJPEG(path string, width, height int) error {
	return createTestImage(path, width, height, func(w io.Writer, img image.Image) error {
		return jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
	})
}

// Helper function to create test GIF images
func createTestGIF(path string, width, height int) error {
	return createTestImage(path, width, height, func(w io.Writer, img image.Image) error {
		return gif.Encode(w, img, nil)
	})
}

func createTestImage(path string, width, height int, encode func(io.Writer, image.Image) error) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a gradient pattern for visual distinctiveness
	for y := range height {
		for x := range width {
			r := uint8((x * 255) / width)                  //nolint:gosec // test code, values bounded by image dimensions
			g := uint8((y * 255) / height)                 //nolint:gosec // test code, values bounded by image dimensions
			b := uint8(((x + y) * 255) / (width + height)) //nolint:gosec // test code, values bounded by image dimensions
			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return encode(file, img)
}
