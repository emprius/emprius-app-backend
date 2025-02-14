package api

import (
	"bytes"
	"image"
	"image/png"
	"testing"
)

func TestCreateThumbnail(t *testing.T) {
	tests := []struct {
		name           string
		width, height  int
		expectedRatio  float64
		expectedWidth  int
		expectedHeight int
	}{
		{
			name:           "Square image",
			width:          500,
			height:         500,
			expectedRatio:  2.0,
			expectedWidth:  maxThumbnailSize,
			expectedHeight: maxThumbnailSize / 2,
		},
		{
			name:           "Wide image (3:1)",
			width:          900,
			height:         300,
			expectedRatio:  2.0,
			expectedWidth:  maxThumbnailSize,
			expectedHeight: maxThumbnailSize / 2,
		},
		{
			name:           "Tall image (1:2)",
			width:          300,
			height:         600,
			expectedRatio:  2.0,
			expectedWidth:  maxThumbnailSize,
			expectedHeight: maxThumbnailSize / 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test image with a pattern to verify cropping
			img := image.NewRGBA(image.Rect(0, 0, tt.width, tt.height))
			// Draw a diagonal line to verify cropping behavior
			for x := 0; x < tt.width; x++ {
				for y := 0; y < tt.height; y++ {
					if x == y || x == tt.width-y {
						img.Set(x, y, image.White)
					}
				}
			}

			var buf bytes.Buffer
			if err := png.Encode(&buf, img); err != nil {
				t.Fatalf("Failed to encode test image: %v", err)
			}

			// Create thumbnail
			thumbnail, err := createThumbnail(buf.Bytes(), "png")
			if err != nil {
				t.Fatalf("createThumbnail failed: %v", err)
			}

			// Decode thumbnail to verify dimensions
			decoded, _, err := image.Decode(bytes.NewReader(thumbnail))
			if err != nil {
				t.Fatalf("Failed to decode thumbnail: %v", err)
			}

			bounds := decoded.Bounds()
			width := bounds.Dx()
			height := bounds.Dy()
			ratio := float64(width) / float64(height)

			if width != tt.expectedWidth {
				t.Errorf("Expected width %d, got %d", tt.expectedWidth, width)
			}
			if height != tt.expectedHeight {
				t.Errorf("Expected height %d, got %d", tt.expectedHeight, height)
			}
			if ratio != tt.expectedRatio {
				t.Errorf("Expected ratio %f, got %f", tt.expectedRatio, ratio)
			}
		})
	}
}
