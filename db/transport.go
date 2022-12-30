package db

import "github.com/rs/zerolog/log"

// Transport is a type that represents a transport.
type Transport struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func createTransportTables(db *Database) error {
	log.Info().Msg("creating transport tables")
	return db.Exec(`
	CREATE TABLE IF NOT EXISTS transport (
		id    INTEGER PRIMARY KEY,
		name  TEXT NOT NULL UNIQUE
	)`)
}
