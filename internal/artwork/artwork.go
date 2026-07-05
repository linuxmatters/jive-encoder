// Package artwork scales podcast cover art to Apple Podcasts specifications.
// The encoder embeds the scaled bytes as an attached-picture stream for
// cover-capable formats.
package artwork

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoder for image.Decode
	_ "image/jpeg" // register JPEG decoder for image.Decode
	"image/png"
	"os"

	"golang.org/x/image/draw"
)

// ScaleCoverArt scales cover art according to Apple Podcasts specifications:
//   - Images < 1400x1400: upscale to 1400x1400
//   - Images 1400x1400 to 3000x3000: use as-is (no scaling artefacts)
//   - Images > 3000x3000: downscale to 3000x3000
//
// It accepts PNG, JPEG, and GIF input. To avoid needless recompression an
// in-spec PNG returns its original bytes untouched; scaled images and non-PNG
// inputs re-encode to PNG.
func ScaleCoverArt(inputPath string) ([]byte, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read cover art: %w", err)
	}

	img, format, err := image.Decode(bytes.NewReader(data))
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
	var needsScaling bool

	// No lower sanity bound is deliberate: any square smaller than 1400 upscales
	// to 1400, so even a 1x1 input is accepted and blown up to spec.
	switch {
	case width < 1400:
		targetSize = 1400
		needsScaling = true
	case width > 3000:
		targetSize = 3000
		needsScaling = true
	}

	// Fast path: an in-spec PNG passes through with its bytes intact.
	if !needsScaling && format == "png" {
		return data, nil
	}

	src := img
	if needsScaling {
		dst := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))

		// Bilinear matches the scaler used by Jivefire thumbnail generation.
		// draw.Src writes the resized pixels straight into the fresh
		// destination, the cheaper choice for a full-frame resize.
		draw.BiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Src, nil)

		src = dst
	}

	// Normalise every re-encoded path to PNG for a consistent attached-picture payload.
	var buf bytes.Buffer

	err = png.Encode(&buf, src)
	if err != nil {
		return nil, fmt.Errorf("failed to encode scaled image: %w", err)
	}

	return buf.Bytes(), nil
}
