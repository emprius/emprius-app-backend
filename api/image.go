package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"  // Import image decoders for supported formats
	_ "image/jpeg" // JPEG support
	_ "image/png"  // PNG support

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

// BinaryResponse represents a binary response that should be sent directly to the client
type BinaryResponse struct {
	ContentType string
	Data        []byte
}

// GET /image/:hash returns the image with the given hash.
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
	return &BinaryResponse{
		ContentType: contentType,
		Data:        image.Content,
	}, nil
}
