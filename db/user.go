package db

import "github.com/rs/zerolog/log"

// User is a type that represents a user of the app.
type User struct {
	ID       int64    `json:"id"`
	Name     string   `json:"name"`
	Email    string   `json:"email"`
	Tokens   uint64   `json:"tokens"`
	Active   bool     `json:"active"`
	Rating   int32    `json:"rating"`
	Avatar   string   `json:"avatar"`
	Location Location `json:"location"`
}

func createUserTables(db *Database) error {
	log.Info().Msg("creating user tables")
	if err := db.Exec(`
	CREATE TABLE IF NOT EXISTS user (
		id        INTEGER PRIMARY KEY,
		name      TEXT NOT NULL,
		email     TEXT NOT NULL UNIQUE,
		tokens    INTEGER,
		active    BOOLEAN,
		rating    INTEGER,
		avatar    TEXT,
		location  (
			latitude INTEGER NOT NULL,
			longitude INTEGER NOT NULL
		),
		CHECK(rating >= 0 AND rating <= 100)
	)`); err != nil {
		return err
	}
	return db.Exec(`
		CREATE INDEX IF NOT EXISTS user_name ON user (name)
	`)
}
