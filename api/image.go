package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"

	stdDraw "image/draw"

	"golang.org/x/image/draw"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/types"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// checkIfDataIsAnImage checks if the given data is an image.
func checkIfDataIsAnImage(data []byte) error {
	if len(data) == 0 {
		return ErrInvalidImageFormat.WithErr(fmt.Errorf("empty image data"))
	}

	// Create a reader from the byte slice
	reader := bytes.NewReader(data)

	// Decode the image
	_, format, err := image.Decode(reader)
	if err != nil {
		return ErrInvalidImageFormat.WithErr(err)
	}

	// Ensure the format is not empty
	if format == "" {
		return ErrInvalidImageFormat.WithErr(fmt.Errorf("unknown image format"))
	}

	return nil
}

// addImage returns the corresponding db.Image to the data content.
// If the image is not in the database, it will be added.
// If the image is already in the database, it will be returned.
func (a *API) addImage(name string, data []byte) (*db.Image, error) {
	if err := checkIfDataIsAnImage(data); err != nil {
		log.Debug().Err(err).Msg("invalid image format")
		return nil, err
	}
	hash := sha256.Sum256(data)
	image, err := a.database.ImageService.GetImage(context.Background(), hash[:])
	if err == mongo.ErrNoDocuments {
		image := &db.Image{
			Hash:    hash[:],
			Content: data,
			Name:    name,
		}
		_, err := a.database.ImageService.InsertImage(context.Background(), image)
		if err != nil {
			return nil, ErrCouldNotInsertToDatabase.WithErr(err)
		}
		log.Debug().Msgf("added image %s", image.Hash.String())
		return image, nil
	}
	if err != nil {
		return nil, ErrInternalServerError.WithErr(err)
	}
	return image, nil
}

func (a *API) image(hash []byte) (*db.Image, error) {
	image, err := a.database.ImageService.GetImage(context.Background(), hash)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrImageNotFound.WithErr(fmt.Errorf("image with hash %x not found", hash))
		}
		return nil, ErrInternalServerError.WithErr(err)
	}
	return image, nil
}

func (a *API) imageListFromSlice(hashes []types.HexBytes) ([]db.Image, error) {
	var images []db.Image
	ctx := context.Background()
	for _, hash := range hashes {
		image, err := a.database.ImageService.GetImage(ctx, hash)
		if err != nil {
			if err == mongo.ErrNoDocuments {
				return nil, ErrImageNotFound.WithErr(fmt.Errorf("image with hash %x not found", hash))
			}
			return nil, ErrInternalServerError.WithErr(err)
		}
		images = append(images, *image)
	}
	return images, nil
}

// POST /image uploads an image to the server.
func (a *API) imageUploadHandler(r *Request) (interface{}, error) {
	if r.UserID == "" {
		return nil, ErrUnauthorized.WithErr(fmt.Errorf("user not authenticated"))
	}

	image := db.Image{}
	if err := json.Unmarshal(r.Data, &image); err != nil {
		return nil, ErrInvalidJSON.WithErr(err)
	}

	if len(image.Content) == 0 {
		return nil, ErrInvalidImageFormat.WithErr(fmt.Errorf("empty image content"))
	}

	dbImage, err := a.addImage(image.Name, image.Content)
	if err != nil {
		return nil, err
	}

	return dbImage, nil
}

const maxThumbnailSize = 512

// createThumbnail generates a thumbnail version of the image with max dimension of 512px
func createThumbnail(imgBytes []byte, format string) ([]byte, error) {
	// Decode original image
	src, _, err := image.Decode(bytes.NewReader(imgBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Calculate new dimensions
	bounds := src.Bounds()
	ratio := float64(bounds.Dx()) / float64(bounds.Dy())
	var width, height int
	if bounds.Dx() > bounds.Dy() {
		width = maxThumbnailSize
		height = int(float64(maxThumbnailSize) / ratio)
	} else {
		height = maxThumbnailSize
		width = int(float64(maxThumbnailSize) * ratio)
	}

	// Create new image with calculated dimensions
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// Scale the image using high-quality algorithm
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), stdDraw.Over, nil)

	// Encode the thumbnail
	var buf bytes.Buffer
	switch format {
	case "jpeg":
		err = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85})
	case "png":
		err = png.Encode(&buf, dst)
	case "gif":
		err = gif.Encode(&buf, dst, nil)
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return buf.Bytes(), nil
}

// GET /image/:hash returns the image with the given hash.
// Supports optional thumbnail parameter to return a resized version.
func (a *API) imageHandler(r *Request) (interface{}, error) {
	hash := r.Context.URLParam("hash")
	if hash == nil {
		return nil, ErrInvalidRequestBodyData.WithErr(fmt.Errorf("missing hash"))
	}
	hashBytes, err := hex.DecodeString(hash[0])
	if err != nil {
		return nil, ErrInvalidHash.WithErr(err)
	}

	image, err := a.image(hashBytes)
	if err != nil {
		return nil, err
	}

	// Get the format from the first few bytes of the image
	format := ""
	if len(image.Content) > 2 {
		if bytes.HasPrefix(image.Content, []byte{0xFF, 0xD8, 0xFF}) {
			format = "jpeg"
		} else if bytes.HasPrefix(image.Content, []byte{0x89, 0x50, 0x4E, 0x47}) {
			format = "png"
		} else if bytes.HasPrefix(image.Content, []byte{0x47, 0x49, 0x46}) {
			format = "gif"
		}
	}
	if format == "" {
		return nil, ErrInvalidImageFormat.WithErr(fmt.Errorf("unsupported image format"))
	}

	contentType := fmt.Sprintf("image/%s", format)
	data := image.Content

	// Check if thumbnail is requested
	if thumbnail := r.Context.URLParam("thumbnail"); thumbnail != nil && thumbnail[0] == "true" {
		thumbnailData, err := createThumbnail(data, format)
		if err != nil {
			return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to create thumbnail: %w", err))
		}
		data = thumbnailData
	}

	return &BinaryResponse{
		ContentType: contentType,
		Data:        data,
	}, nil
}
