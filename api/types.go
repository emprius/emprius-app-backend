package api

import (
	"time"

	"github.com/emprius/emprius-app-backend/db"
)

type Login struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
type LoginResponse struct {
	Token    string    `json:"token"`
	Expirity time.Time `json:"expirity"`
}

type Register struct {
	UserEmail         string `json:"email"`
	RegisterAuthToken string `json:"invitationToken"`
	UserProfile
}

type UserProfile struct {
	Name      string       `json:"name"`
	Community string       `json:"community"`
	Location  *db.Location `json:"location"`
	Active    *bool        `json:"active"`
	Avatar    []byte       `json:"avatar"`
	Password  string       `json:"password"`
}
