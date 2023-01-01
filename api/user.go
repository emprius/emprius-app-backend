package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"net/http"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/genjidb/genji/document"
	"github.com/rs/zerolog/log"
)

// registerHandler handles the register request. It creates a new user in the database.
func (a *API) registerHandler(w http.ResponseWriter, r *http.Request) {
	userInfo := Register{}
	if err := json.NewDecoder(r.Body).Decode(&userInfo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if userInfo.RegisterAuthToken != a.registerAuthToken {
		http.Error(w, "invalid register auth token", http.StatusUnauthorized)
		return
	}
	hashedPassword := sha256.New().Sum([]byte(passwordSalt + userInfo.Password))
	user := db.User{
		Email:    userInfo.UserEmail,
		Password: hashedPassword,
		Name:     userInfo.Name,
		Avatar:   userInfo.Avatar,
		Location: userInfo.Location,
		Active:   true,
		Rating:   50,
		Tokens:   1000,
	}
	log.Debug().Msgf("adding user %+v", user)
	if err := a.database.Exec("INSERT INTO user VALUES ?", &user); err != nil {
		log.Error().Err(err).Msg("failed to insert user")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

// loginHandler handles the login request. It returns a JWT token if the login is successful.
func (a *API) loginHandler(w http.ResponseWriter, r *http.Request) {
	// Get the user name from the request body
	loginInfo := Login{}
	if err := json.NewDecoder(r.Body).Decode(&loginInfo); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	stream, err := a.database.QueryDocument("SELECT * FROM user WHERE email = ?", loginInfo.Email)
	if err != nil {
		http.Error(w, "wrong password or email", http.StatusUnauthorized)
		return
	}

	user := db.User{}
	if err := document.StructScan(stream, &user); err != nil {
		log.Err(err).Msg("failed to scan user")
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	hashedPassword := sha256.New().Sum([]byte(passwordSalt + loginInfo.Password))
	if !bytes.Equal(user.Password, hashedPassword) {
		http.Error(w, "wrong password or email", http.StatusUnauthorized)
		log.Debug().Msgf("passwords don't match for userId %s", user.Email)
		return
	}
	// Generate a new token with the user name as the subject
	token, err := a.makeToken(user.Email)
	if err != nil {
		log.Error().Err(err).Msg("failed to generate token")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data, err := json.Marshal(&token)
	if err != nil {
		log.Error().Err(err).Msg("could not marshal token")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	// Write the token to the HTTP response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		log.Error().Err(err).Msg("could not write response body")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (a *API) userProfileHandler(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-Id")
	log.Debug().Msgf("profile for user id %s", userID)
	stream, err := a.database.QueryDocument("SELECT * FROM user WHERE email = ?", userID)
	if err != nil {
		log.Error().Err(err).Msg("failed to query user profile")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	user := db.User{}
	if err := document.StructScan(stream, &user); err != nil {
		log.Error().Err(err).Msg("failed to scan user profile")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	data, err := json.Marshal(&user)
	if err != nil {
		log.Error().Err(err).Msg("failed to marshal user profile")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(data); err != nil {
		log.Error().Err(err).Msg("failed to write response")
		return
	}
}
