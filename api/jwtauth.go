package api

import (
	"context"
	"fmt"
	"golang.org/x/crypto/argon2"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

// authHandler is a handler that authenticates the user and returns a JWT token.
// If successful, the user identifier is added to the HTTP header as `X-User-Id`,
// so that it can be used by the next handlers.
func (a *API) authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil || token == nil {
			http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
			return
		}

		// Get userId from claims
		userId, ok := claims["userId"].(string)
		if !ok {
			http.Error(w, ErrUnauthorized.Error(), http.StatusUnauthorized)
			return
		}

		// Validate userId format
		if err := validateObjectID(userId); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Add validated userId to header
		r.Header.Add("X-User-Id", userId)
		// Token is authenticated, pass it through
		next.ServeHTTP(w, r)
	})
}

// makeToken creates a JWT token for the given user identifier.
// The token is signed with the API secret, following the JWT specification.
// The token is valid for the period specified on jwtExpiration constant.
func (a *API) makeToken(id string) (*LoginResponse, error) {
	j := jwt.New()
	if err := j.Set("userId", id); err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to set userId claim: %w", err))
	}
	if err := j.Set(jwt.ExpirationKey, time.Now().Add(jwtExpiration).Unix()); err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to set expiration claim: %w", err))
	}
	lr := LoginResponse{}
	lr.Expirity = time.Now().Add(jwtExpiration)
	jmap, err := j.AsMap(context.Background())
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to convert token to map: %w", err))
	}
	_, lr.Token, _ = a.auth.Encode(jmap)
	return &lr, nil
}

// HashPassword helper function allows to hash a password using a salt.
func hashPassword(password string) []byte {
	return argon2hash([]byte(password), []byte(passwordSalt))
}

func argon2hash(data, salt []byte) []byte {
	// Argon2 parameters for hashing, if modified, the current hashes will be invalidated
	memory := uint32(64 * 1024)
	argonTime := uint32(4)
	argonThreads := uint8(8)
	return argon2.IDKey([]byte(data), []byte(salt), argonTime, memory, argonThreads, 32)
}
