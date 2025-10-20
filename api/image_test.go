package api

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"
)

func TestExtractExifOrientation(t *testing.T) {
	tests := []struct {
		name        string
		imageData   []byte
		expected    int
		description string
	}{
		{
			name:        "No EXIF data",
			imageData:   createSimplePNGImage(100, 100),
			expected:    1,
			description: "PNG without EXIF should return 1 (normal)",
		},
		{
			name:        "Empty data",
			imageData:   []byte{},
			expected:    1,
			description: "Empty data should return 1 (normal)",
		},
		{
			name:        "Invalid image data",
			imageData:   []byte{0x00, 0x01, 0x02, 0x03},
			expected:    1,
			description: "Invalid data should return 1 (normal)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			orientation := extractExifOrientation(tt.imageData)
			if orientation != tt.expected {
				t.Errorf("%s: expected orientation %d, got %d", tt.description, tt.expected, orientation)
			}
		})
	}
}

func TestCopyExifOrientationToJPEG(t *testing.T) {
	// Create a simple JPEG thumbnail
	img := image.NewRGBA(image.Rect(0, 0, 100, 50))
	for y := 0; y < 50; y++ {
		for x := 0; x < 100; x++ {
			intensity := uint8(float64(x) / 100.0 * 255)
			img.Set(x, y, color.Gray{intensity})
		}
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
		t.Fatalf("Failed to encode test JPEG: %v", err)
	}
	thumbnailData := buf.Bytes()

	tests := []struct {
		name        string
		orientation int
		shouldCopy  bool
	}{
		{
			name:        "Orientation 1 (normal) - no copy",
			orientation: 1,
			shouldCopy:  false,
		},
		{
			name:        "Orientation 6 (90 CW) - should copy",
			orientation: 6,
			shouldCopy:  true,
		},
		{
			name:        "Orientation 8 (270 CW) - should copy",
			orientation: 8,
			shouldCopy:  true,
		},
		{
			name:        "Invalid orientation 0 - no copy",
			orientation: 0,
			shouldCopy:  false,
		},
		{
			name:        "Invalid orientation 9 - no copy",
			orientation: 9,
			shouldCopy:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := copyExifOrientationToJPEG(thumbnailData, tt.orientation)
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if result == nil {
				t.Fatal("Result should not be nil")
			}

			// For orientations that shouldn't be copied, result should be same as input
			if !tt.shouldCopy && !bytes.Equal(result, thumbnailData) {
				t.Errorf("Expected unchanged data for orientation %d", tt.orientation)
			}

			// Verify result is still a valid JPEG
			_, _, err = image.Decode(bytes.NewReader(result))
			if err != nil {
				t.Errorf("Result is not a valid JPEG: %v", err)
			}

			// For orientations that should be copied, verify EXIF exists
			if tt.shouldCopy {
				resultOrientation := extractExifOrientation(result)
				if resultOrientation != tt.orientation {
					t.Errorf("Expected orientation %d in result, got %d", tt.orientation, resultOrientation)
				}
			}
		})
	}
}

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

func TestCreateThumbnailWithOrientation(t *testing.T) {
	// Create a tall portrait image (600x900)
	width, height := 600, 900
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// Create a vertical gradient
			intensity := uint8(float64(y) / float64(height) * 255)
			img.Set(x, y, color.RGBA{intensity, intensity, intensity, 255})
		}
	}

	// Encode as JPEG (PNG doesn't support EXIF in standard Go library)
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}

	// Create thumbnail
	thumbnail, err := createThumbnail(buf.Bytes(), "jpeg")
	if err != nil {
		t.Fatalf("createThumbnail failed: %v", err)
	}

	// Decode thumbnail to verify it was created successfully
	decoded, _, err := image.Decode(bytes.NewReader(thumbnail))
	if err != nil {
		t.Fatalf("Failed to decode thumbnail: %v", err)
	}

	bounds := decoded.Bounds()
	thumbWidth := bounds.Dx()
	thumbHeight := bounds.Dy()

	// Verify thumbnail has correct 2:1 aspect ratio
	if thumbWidth != maxThumbnailSize || thumbHeight != maxThumbnailSize/2 {
		t.Errorf("Expected thumbnail dimensions %dx%d, got %dx%d",
			maxThumbnailSize, maxThumbnailSize/2, thumbWidth, thumbHeight)
	}
}

// Helper function to create a simple PNG image for testing
func createSimplePNGImage(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// Fill with gray
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.Gray{128})
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
