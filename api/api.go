package api

import (
	"fmt"
	"net/http"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"
	"github.com/lestrrat-go/jwx/v2/jwt"

	"github.com/rs/zerolog/log"
)

// API type represents the API HTTP server with JWT authentication capabilities.
type API struct {
	Router   *chi.Mux
	auth     *jwtauth.JWTAuth
	database *db.Database
}

// New creates a new API HTTP server. It does not start the server. Use Start() for that.
func New(secret string, database *db.Database) *API {
	return &API{
		auth:     jwtauth.New("HS256", []byte(secret), nil),
		database: database,
	}
}

// Start starts the API HTTP server (non blocking).
func (a *API) Start(host string, port int) {
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), a.router()); err != nil {
			log.Fatal().Err(err).Msg("failed to start api router")
		}
	}()
}

func (a *API) authenticator(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, claims, err := jwtauth.FromContext(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		if token == nil || jwt.Validate(token, jwt.WithRequiredClaim("userId")) != nil {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}
		// Retrieve the `userId` from the claims and add it to the HTTP header
		w.Header().Add("X-User-Id", claims["userId"].(string))
		// Token is authenticated, pass it through
		next.ServeHTTP(w, r)
	})
}

func (a *API) makeToken(name string) string {
	_, tokenString, _ := a.auth.Encode(map[string]interface{}{"userId": name})
	return tokenString
}

func (a *API) router() http.Handler {
	// For debugging/example purposes, we generate and print
	// a sample jwt token with claims `userId:123` here:
	log.Debug().Msgf("a sample jwt is %s", a.makeToken("123"))

	// Create the router with a basic middleware stack
	r := chi.NewRouter()
	r.Use(cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}).Handler)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Throttle(100))

	// Protected routes
	r.Group(func(r chi.Router) {
		// Seek, verify and validate JWT tokens
		r.Use(jwtauth.Verifier(a.auth))

		// Handle valid / invalid tokens. In this example, we use
		// the provided authenticator middleware, but you can write your
		// own very easily, look at the Authenticator method in jwtauth.go
		// and tweak it, its not scary.
		r.Use(a.authenticator)

		r.Get("/admin", func(w http.ResponseWriter, r *http.Request) {
			_, claims, _ := jwtauth.FromContext(r.Context())
			if _, err := w.Write([]byte(fmt.Sprintf("protected area. hi %v", claims["user_id"]))); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
		})
	})

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte("welcome anonymous")); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
		})
	})

	return r
}
