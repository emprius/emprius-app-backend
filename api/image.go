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
		return fmt.Errorf("data is empty")
	}

	// Create a reader from the byte slice
	reader := bytes.NewReader(data)

	// Decode the image
	_, format, err := image.Decode(reader)
	if err != nil {
		return fmt.Errorf("could not decode: %w", err)
	}

	// Ensure the format is not empty
	if format == "" {
		return fmt.Errorf("could not determine format")
	}

	return nil
}

// addImage returns the corresponding db.Image to the data content.
// If the image is not in the database, it will be added.
// If the image is already in the database, it will be returned.
func (a *API) addImage(name string, data []byte) (*db.Image, error) {
	if err := checkIfDataIsAnImage(data); err != nil {
		log.Debug().Err(err).Msg("invalid image format")
		return nil, ErrInvalidImageFormat
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
			return nil, fmt.Errorf("could not insert image to database: %w", err)
		}
		log.Debug().Msgf("added image %s", image.Hash.String())
		return image, nil
	}
	if err != nil {
		return nil, err
	}
	return image, nil
}

func (a *API) image(hash []byte) (*db.Image, error) {
	image, err := a.database.ImageService.GetImage(context.Background(), hash)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, ErrImageNotFound
		}
		return nil, err
	}
	return image, nil
}

func (a *API) imageListFromSlice(hashes []types.HexBytes) ([]db.Image, error) {
	var images []db.Image
	ctx := context.Background()
	for _, hash := range hashes {
		image, err := a.database.ImageService.GetImage(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("could not get image: %x", hash)
		}
		images = append(images, *image)
	}
	return images, nil
}

// POST /image uploads an image to the server.
func (a *API) imageUploadHandler(r *Request) (interface{}, error) {
	image := db.Image{}
	if err := json.Unmarshal(r.Data, &image); err != nil {
		return nil, ErrInvalidJSON
	}
	dbImage, err := a.addImage(image.Name, image.Content)
	if err != nil {
		return nil, fmt.Errorf("could not add image: %w", err)
	}

	return dbImage, nil
}

// GET /image/:hash returns the image with the given hash.
func (a *API) imageHandler(r *Request) (interface{}, error) {
	hash := r.Context.URLParam("hash")
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, ErrInvalidHash
	}
	return a.image(hashBytes)
}
