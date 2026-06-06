package id3

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

// ScaleCoverArt scales cover art according to Apple Podcasts specifications:
//   - Images < 1400x1400: upscale to 1400x1400
//   - Images 1400x1400 to 3000x3000: use as-is (no scaling artifacts)
//   - Images > 3000x3000: downscale to 3000x3000
//
// To avoid needless recompression it returns the original PNG bytes untouched
// when no scaling is required, and only re-encodes scaled images or non-PNG
// inputs.
func ScaleCoverArt(inputPath string) ([]byte, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open cover art: %w", err)
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode cover art: %w", err)
	}

	// Apple Podcasts requires square artwork.
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width != height {
		return nil, fmt.Errorf("cover art must be square (got %dx%d)", width, height)
	}

	var targetSize int
	needsScaling := false

	switch {
	case width < 1400:
		targetSize = 1400
		needsScaling = true
	case width >= 1400 && width <= 3000:
		targetSize = width
		needsScaling = false
	case width > 3000:
		targetSize = 3000
		needsScaling = true
	}

	// Fast path: an in-spec PNG passes through with its bytes intact.
	if !needsScaling && format == "png" {
		data, err := os.ReadFile(inputPath)
		if err != nil {
			return nil, fmt.Errorf("failed to read cover art: %w", err)
		}

		return data, nil
	}

	var finalImg image.Image
	if needsScaling {
		dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))

		// Bilinear matches the scaler used by Jivefire thumbnail generation.
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)

		finalImg = dst
	} else {
		// Reaches here only for an in-spec non-PNG, re-encoded below.
		finalImg = img
	}

	// Normalise every re-encoded path to PNG for a consistent APIC payload.
	var buf bytes.Buffer

	err = png.Encode(&buf, finalImg)
	if err != nil {
		return nil, fmt.Errorf("failed to encode scaled image: %w", err)
	}

	return buf.Bytes(), nil
}
