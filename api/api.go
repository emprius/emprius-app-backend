package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/emprius/emprius-app-backend/notifications"
	"github.com/rs/zerolog"

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

type APIConfig struct {
	DB                 *db.Database
	MailService        notifications.NotificationService
	JwtSecret          string
	RegisterToken      string
	MaxInviteCodes     int
	InviteCodeCooldown int
	Debug              bool
}

// API type represents the API HTTP server with JWT authentication capabilities.
type API struct {
	db                 *db.Database
	mail               notifications.NotificationService
	Router             *chi.Mux
	auth               *jwtauth.JWTAuth
	registerAuthToken  string
	database           *db.Database
	webappdir          string
	maxInviteCodes     int
	inviteCodeCooldown int
	rateLimiter        *MessageRateLimiter
}

// New creates a new API HTTP server. It does not start the server. Use Start() for that.
func New(conf *APIConfig) (*API, error) {
	if conf == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if conf.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Info().Msg("starting app backend")

	webappdir := os.Getenv("WEBAPPDIR")
	if webappdir == "" {
		log.Warn().Msg("WEBAPPDIR not set, static files will not be served")
	}

	return &API{
		auth:               jwtauth.New("HS256", []byte(conf.JwtSecret), nil),
		database:           conf.DB,
		registerAuthToken:  conf.RegisterToken,
		webappdir:          webappdir,
		maxInviteCodes:     conf.MaxInviteCodes,
		inviteCodeCooldown: conf.InviteCodeCooldown,
		mail:               conf.MailService,
		rateLimiter:        NewMessageRateLimiter(),
	}, nil
}

// Start starts the API HTTP server (non blocking).
func (a *API) Start(host string, port int) {
	go func() {
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), a.router()); err != nil {
			log.Fatal().Err(err).Msg("failed to start api router")
		}
		log.Info().Msgf("api service started at %s:%d", host, port)
	}()
}

// Close closes the API service database.
func (a *API) Close() {
	if err := a.db.Close(context.Background()); err != nil {
		log.Warn().Err(err).Msg("failed to close database")
	}
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

		// Register domain-specific routes
		a.RegisterUserRoutes(r)
		a.RegisterToolRoutes(r)
		a.RegisterBookingRoutes(r)
		a.RegisterCommunityRoutes(r)
		a.RegisterImageRoutes(r)
		a.RegisterMessageRoutes(r)
	})

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(".")); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
		})

		// Register public domain-specific routes
		a.RegisterPublicUserRoutes(r)
		a.RegisterPublicImageRoutes(r)

		// Info route
		log.Info().Msg("register route GET /info")
		r.Get("/info", a.routerHandler(a.infoHandler))

		// Static handler for webappdir (testing)
		log.Info().Msg("register route GET /app/*")
		r.Get("/app*", a.staticHandler)
	})

	return r
}

// staticHandler serves the static files from the webappdir directory, for testing purposes.
func (a *API) staticHandler(w http.ResponseWriter, r *http.Request) {
	if a.webappdir == "" {
		http.Error(w, "webappdir not set", http.StatusInternalServerError)
		return
	}
	var filePath string
	if r.URL.Path == "/app" || r.URL.Path == "/app/" {
		filePath = path.Join(a.webappdir, "index.html")
	} else {
		filePath = path.Join(a.webappdir, strings.TrimPrefix(path.Clean(r.URL.Path), "/app"))
	}
	// Serve the file using http.ServeFile
	http.ServeFile(w, r, filePath)
}

// info handler returns the basic info about the API.
func (a *API) infoHandler(r *Request) (interface{}, error) {
	ctx := context.Background()

	// Get user count
	userCount, err := a.database.UserService.CountUsers(ctx)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to count users: %w", err))
	}

	// Get tool count
	toolCount, err := a.database.ToolService.CountTools(ctx)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to count tools: %w", err))
	}

	// Get all transports
	transports, err := a.database.TransportService.GetAllTransports(ctx)
	if err != nil {
		return nil, ErrInternalServerError.WithErr(fmt.Errorf("failed to get transports: %w", err))
	}

	// Convert *Transport slice to Transport slice
	transportList := make([]db.Transport, len(transports))
	for i, t := range transports {
		transportList[i] = *t
	}

	// Get categories
	categories := a.toolCategories()

	return &Info{
		Users:      int(userCount),
		Tools:      int(toolCount),
		Categories: categories,
		Transports: transportList,
	}, nil
}
