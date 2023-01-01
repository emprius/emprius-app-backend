package db

import "github.com/rs/zerolog/log"

// User is a type that represents a user of the app.
type User struct {
	Email      string   `json:"email"`
	Name       string   `json:"name"`
	Community  string   `json:"community"`
	Password   []byte   `json:"password"`
	Tokens     uint64   `json:"tokens"`
	Active     bool     `json:"active"`
	Rating     int32    `json:"rating"`
	AvatarHash HexBytes `json:"avatarHash" genji:"avatarHash"` // hash of the image
	Location   Location `json:"location"`
	Verified   bool     `json:"-"`
}

func createUserTables(db *Database) error {
	log.Info().Msg("creating user tables")
	if err := db.Exec(`
	CREATE TABLE IF NOT EXISTS user (
		email     TEXT NOT NULL PRIMARY KEY,
		name      TEXT NOT NULL UNIQUE,
		community TEXT,
		password  BLOB NOT NULL,
		tokens    INTEGER DEFAULT 1000,
		active    BOOLEAN DEFAULT TRUE,
		rating    INTEGER DEFAULT 50,
		avatarHash BLOB,
		verified  BOOLEAN DEFAULT FALSE,
		location  (
			latitude INTEGER NOT NULL,
			longitude INTEGER NOT NULL
		),
		CONSTRAINT rating_check CHECK(rating >= 0 AND rating <= 100),
		CONSTRAINT length_check CHECK(  
			len(name) > 5 AND 
			len(name) < 30 AND 
			len(email) > 8 AND 
			len(email) < 30)
		)`); err != nil {
		return err
	}
	return db.Exec(`
		CREATE INDEX IF NOT EXISTS user_name ON user (name);
	`)
}
