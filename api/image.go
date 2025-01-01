package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog/log"
	"go.mongodb.org/mongo-driver/mongo"
)

// addImage returns the corresponding db.Image to the data content.
// If the image is not in the database, it will be added.
// If the image is already in the database, it will be returned.
func (a *API) addImage(name string, data []byte) (*db.Image, error) {
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
		log.Debug().Msgf("added image %s", base64.StdEncoding.EncodeToString(image.Hash))
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

func (a *API) imageListFromSlice(hashes []HexBytes) ([]*Image, error) {
	var images []*Image
	ctx := context.Background()
	for _, hash := range hashes {
		image, err := a.database.ImageService.GetImage(ctx, hash)
		if err != nil {
			return nil, fmt.Errorf("could not get image: %x", hash)
		}
		images = append(images, &Image{Hash: image.Hash, Name: image.Name})
	}
	return images, nil
}

// POST /image uploads an image to the server.
func (a *API) imageUploadHandler(r *Request) (interface{}, error) {
	image := Image{}
	if err := json.Unmarshal(r.Data, &image); err != nil {
		return nil, ErrInvalidJSON
	}
	dbImage, err := a.addImage(image.Name, image.Data)
	if err != nil {
		return nil, fmt.Errorf("could not add image: %w", err)
	}

	return &Image{Hash: dbImage.Hash}, nil
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
