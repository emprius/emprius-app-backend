package api

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/genjidb/genji"
	"github.com/genjidb/genji/document"
	"github.com/rs/zerolog/log"
)

// addImage returns the corresponding db.Image to the data content.
// If the image is not in the database, it will be added.
// If the image is already in the database, it will be returned.
func (a *API) addImage(name string, data []byte) (*db.Image, error) {
	hash := sha256.Sum256(data)
	doc, err := a.database.QueryDocument("SELECT * FROM image WHERE hash = ?", hash[:])
	if genji.IsNotFoundError(err) {
		image := db.Image{
			Hash:    hash[:],
			Content: data,
			Name:    name,
		}
		if err := a.database.Exec("INSERT INTO image VALUES ?", &image); err != nil {
			return nil, fmt.Errorf("could not insert image to database: %w", err)
		}
		log.Debug().Msgf("added image %s", base64.StdEncoding.EncodeToString(image.Hash))
		return &image, nil
	}
	if err != nil {
		return nil, err
	}
	image := db.Image{}
	if err := document.StructScan(doc, &image); err != nil {
		return nil, fmt.Errorf("failed to scan image: %w", err)
	}
	return &image, nil
}

func (a *API) image(r *Request) (interface{}, error) {
	hash := r.Context.URLParam("hash")
	hashBytes, err := hex.DecodeString(hash)
	if err != nil {
		return nil, ErrInvalidHash
	}
	doc, err := a.database.QueryDocument("SELECT * FROM image WHERE hash = ?", hashBytes)
	if err != nil {
		return nil, err
	}
	image := db.Image{}
	if err := document.StructScan(doc, &image); err != nil {
		return nil, err
	}
	return &image, nil
}
