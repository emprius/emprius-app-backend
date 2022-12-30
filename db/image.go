package db

import "github.com/rs/zerolog/log"

// Image is a type that represents an image.
type Image struct {
	Hash    []byte `json:"hash"`
	Name    string `json:"name"`
	Content []byte `json:"content"`
	Link    string `json:"link"`
}

func createImageTable(db *Database) error {
	log.Info().Msg("creating image table")
	return db.Exec(`
	CREATE TABLE IF NOT EXISTS image (
		hash         BLOB PRIMARY KEY,
		name         TEXT,
		content      BLOB NOT NULL,
		link 		 TEXT
	)`)
}
