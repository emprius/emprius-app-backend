package db

import "github.com/rs/zerolog/log"

var initialTransports = []string{
	"Car",
	"Van",
	"Truck",
}

// Transport is a type that represents a transport.
type Transport struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func createTransportTables(db *Database) error {
	log.Info().Msg("creating transport tables")
	if err := db.Exec(`
	CREATE TABLE IF NOT EXISTS transport (
		id    INTEGER PRIMARY KEY,
		name  TEXT NOT NULL UNIQUE
	)`); err != nil {
		return err
	}
	if _, err := db.QueryDocument("SELECT id FROM transport"); err == nil {
		return nil
	}
	for i, t := range initialTransports {
		if err := db.Exec(`INSERT INTO transport (id,name) VALUES (?,?)`, i+1, t); err != nil {
			return err
		}
	}
	return nil
}
