package api

import (
	"fmt"
	"net/http"
	"time"

	"github.com/emprius/emprius-app-backend/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth/v5"

	"github.com/rs/zerolog/log"
)

const (
	jwtExpiration = 720 * time.Hour // 30 days
	passwordSalt  = "emprius"       // salt for password hashing
)

// API type represents the API HTTP server with JWT authentication capabilities.
type API struct {
	Router            *chi.Mux
	auth              *jwtauth.JWTAuth
	registerAuthToken string
	database          *db.Database
}

// New creates a new API HTTP server. It does not start the server. Use Start() for that.
func New(secret, registerAuthToken string, database *db.Database) *API {
	return &API{
		auth:              jwtauth.New("HS256", []byte(secret), nil),
		database:          database,
		registerAuthToken: registerAuthToken,
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

// router creates the router with all the routes and middleware.
func (a *API) router() http.Handler {
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
	r.Use(middleware.ThrottleBacklog(5000, 40000, 30*time.Second))
	r.Use(middleware.Timeout(30 * time.Second))
	// Protected routes
	r.Group(func(r chi.Router) {
		// Seek, verify and validate JWT tokens
		r.Use(jwtauth.Verifier(a.auth))

		// Handle valid JWT tokens.
		r.Use(a.authenticator)

		// Endpoints
		// Users
		log.Info().Msg("register route GET /profile")
		r.Get("/profile", a.routerHandler(a.userProfileHandler))
		log.Info().Msg("register route GET /refresh")
		r.Get("/refresh", a.routerHandler(a.refreshHandler))
		log.Info().Msg("register route POST /profile")
		r.Post("/profile", a.routerHandler(a.userProfileUpdateHandler))

		// Images
		// GET /images/{hash}
		log.Info().Msg("register route GET /images/{hash}")
		r.Get("/images/{hash}", a.routerHandler(a.imageHandler))
		// POST /images
		log.Info().Msg("register route POST /images")
		r.Post("/images", a.routerHandler(a.imageUploadHandler))

		// Tools
		// GET /tools
		log.Info().Msg("register route GET /tools")
		r.Get("/tools", a.routerHandler(a.ownToolsHandler))
		// GET /tools/{id}
		log.Info().Msg("register route GET /tools/{id}")
		r.Get("/tools/{id}", a.routerHandler(a.toolHandler))
		// POST /tools
		log.Info().Msg("register route POST /tools")
		r.Post("/tools", a.routerHandler(a.addToolHandler))
		// PUT /tools/{id}
		log.Info().Msg("register route PUT /tools/{id}")
		r.Put("/tools/{id}", a.routerHandler(a.editToolHandler))
		// DELETE /tools/{id}
		log.Info().Msg("register route DELETE /tools/{id}")
		r.Delete("/tools/{id}", a.routerHandler(a.deleteToolHandler))
		// GET /tools/user/{id}
		log.Info().Msg("register route GET /tools/user/{id}")
		r.Get("/tools/user/{id}", a.routerHandler(a.userToolsHandler))
		// GET /tools/search
		log.Info().Msg("register route GET /tools/search")
		r.Get("/tools/search", a.routerHandler(a.toolSearchHandler))
	})

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(".")); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
		})
		r.Post("/login", a.routerHandler(a.loginHandler))
		r.Post("/register", a.routerHandler(a.registerHandler))
	})

	return r
}
