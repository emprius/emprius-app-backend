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
	UserEmail         string      `json:"email"`
	Password          string      `json:"password"`
	Name              string      `json:"name"`
	Avatar            string      `json:"avatar"`
	Location          db.Location `json:"location"`
	RegisterAuthToken string      `json:"registerAuthToken"`
}
