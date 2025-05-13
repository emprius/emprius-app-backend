package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
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
	Router             *chi.Mux
	auth               *jwtauth.JWTAuth
	registerAuthToken  string
	database           *db.Database
	webappdir          string
	maxInviteCodes     int
	inviteCodeCooldown int
}

// New creates a new API HTTP server. It does not start the server. Use Start() for that.
func New(secret, registerAuthToken string, database *db.Database, maxInviteCodes, inviteCodeCooldown int) *API {
	webappdir := os.Getenv("WEBAPPDIR")
	if webappdir == "" {
		log.Warn().Msg("WEBAPPDIR not set, static files will not be served")
	}
	return &API{
		auth:               jwtauth.New("HS256", []byte(secret), nil),
		database:           database,
		registerAuthToken:  registerAuthToken,
		webappdir:          webappdir,
		maxInviteCodes:     maxInviteCodes,
		inviteCodeCooldown: inviteCodeCooldown,
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
		// POST /profile/invites
		log.Info().Msg("register route POST /profile/invites")
		r.Post("/profile/invites", a.routerHandler(a.userInviteCodesHandler))
		// GET /profile/pendings
		log.Info().Msg("register route GET /profile/pendings")
		r.Get("/profile/pendings", a.routerHandler(a.HandleCountPendingActions))
		log.Info().Msg("register route GET /refresh")
		r.Get("/refresh", a.routerHandler(a.refreshHandler))
		log.Info().Msg("register route GET /profile")
		r.Get("/profile", a.routerHandler(a.userProfileHandler))
		log.Info().Msg("register route POST /profile")
		r.Post("/profile", a.routerHandler(a.userProfileUpdateHandler))
		log.Info().Msg("register route GET /profile/pendings")
		r.Get("/profile/pendings", a.routerHandler(a.HandleCountPendingActions))
		log.Info().Msg("register route GET /users")
		r.Get("/users", a.routerHandler(a.usersHandler))
		log.Info().Msg("register route GET /users/{id}")
		r.Get("/users/{id}", a.routerHandler(a.getUserHandler))
		// GET /users/{id}/ratings
		log.Info().Msg("register route GET /users/{id}/ratings")
		r.Get("/users/{id}/ratings", a.routerHandler(a.HandleGetUserRatings))
		// GET /users/{userId}/communities - Get communities for a specific user
		log.Info().Msg("register route GET /users/{userId}/communities")
		r.Get("/users/{userId}/communities", a.routerHandler(a.getUserCommunitiesHandler))

		// Images
		// POST /images
		log.Info().Msg("register route POST /images")
		r.Post("/images", a.routerHandler(a.imageUploadHandler))

		// Tools
		// GET /tools
		log.Info().Msg("register route GET /tools")
		r.Get("/tools", a.routerHandler(a.ownToolsHandler))
		// GET /tools/search
		log.Info().Msg("register route GET /tools/search")
		r.Get("/tools/search", a.routerHandler(a.toolSearchHandler))
		// GET /tools/user/{id}
		log.Info().Msg("register route GET /tools/user/{id}")
		r.Get("/tools/user/{id}", a.routerHandler(a.userToolsHandler))
		// GET /tools/{id}
		log.Info().Msg("register route GET /tools/{id}")
		r.Get("/tools/{id}", a.routerHandler(a.toolHandler))
		// GET /tools/{id}/ratings
		log.Info().Msg("register route GET /tools/{id}/ratings")
		r.Get("/tools/{id}/ratings", a.routerHandler(a.HandleGetToolRatings))
		// GET /tools/{id}/history
		log.Info().Msg("register route GET /tools/{id}/history")
		r.Get("/tools/{id}/history", a.routerHandler(a.toolHistoryHandler))
		// POST /tools
		log.Info().Msg("register route POST /tools")
		r.Post("/tools", a.routerHandler(a.addToolHandler))
		// PUT /tools/{id}
		log.Info().Msg("register route PUT /tools/{id}")
		r.Put("/tools/{id}", a.routerHandler(a.editToolHandler))
		// DELETE /tools/{id}
		log.Info().Msg("register route DELETE /tools/{id}")
		r.Delete("/tools/{id}", a.routerHandler(a.deleteToolHandler))

		// Bookings
		// POST /bookings
		log.Info().Msg("register route POST /bookings")
		r.Post("/bookings", a.routerHandler(a.HandleCreateBooking))
		// GET /bookings/requests/outgoing
		log.Info().Msg("register route GET /bookings/requests/outgoing")
		r.Get("/bookings/requests/outgoing", a.routerHandler(a.HandleGetOutgoingRequests))
		// GET /bookings/requests/incoming
		log.Info().Msg("register route GET /bookings/requests/incoming")
		r.Get("/bookings/requests/incoming", a.routerHandler(a.HandleGetIncomingRequests))
		// PUT /bookings/{bookingId} - Update booking status
		log.Info().Msg("register route PUT /bookings/{bookingId}")
		r.Put("/bookings/{bookingId}", a.routerHandler(a.HandleUpdateBookingStatus))
		// GET /bookings/{bookingId}
		log.Info().Msg("register route GET /bookings/{bookingId}")
		r.Get("/bookings/{bookingId}", a.routerHandler(a.HandleGetBooking))
		// GET /bookings/ratings/pending
		log.Info().Msg("register route GET /bookings/ratings/pending")
		r.Get("/bookings/ratings/pending", a.routerHandler(a.HandleGetPendingRatings))
		// POST /bookings/{bookingId}/ratings
		log.Info().Msg("register route POST /bookings/{bookingId}/ratings")
		r.Post("/bookings/{bookingId}/ratings", a.routerHandler(a.HandleRateBooking))
		// GET /bookings/{bookingId}/ratings
		log.Info().Msg("register route GET /bookings/{bookingId}/ratings")
		r.Get("/bookings/{bookingId}/ratings", a.routerHandler(a.HandleGetBookingRatings))

		// Communities
		// POST /communities
		log.Info().Msg("register route POST /communities")
		r.Post("/communities", a.routerHandler(a.createCommunityHandler))
		// GET /communities/{communityId}
		log.Info().Msg("register route GET /communities/{communityId}")
		r.Get("/communities/{communityId}", a.routerHandler(a.getCommunityHandler))
		// PUT /communities/{communityId}
		log.Info().Msg("register route PUT /communities/{communityId}")
		r.Put("/communities/{communityId}", a.routerHandler(a.updateCommunityHandler))
		// DELETE /communities/{communityId}
		log.Info().Msg("register route DELETE /communities/{communityId}")
		r.Delete("/communities/{communityId}", a.routerHandler(a.deleteCommunityHandler))
		// GET /communities/{communityId}/members
		log.Info().Msg("register route GET /communities/{communityId}/members")
		r.Get("/communities/{communityId}/members", a.routerHandler(a.getCommunityUsersHandler))
		// GET /communities/{communityId}/tools
		log.Info().Msg("register route GET /communities/{communityId}/tools")
		r.Get("/communities/{communityId}/tools", a.routerHandler(a.getCommunityToolsHandler))
		// POST /communities/{communityId}/members/{userId} - Invite user to community
		log.Info().Msg("register route POST /communities/{communityId}/members/{userId}")
		r.Post("/communities/{communityId}/members/{userId}", a.routerHandler(a.inviteUserToCommunityHandler))
		// DELETE /communities/{communityId}/members} - Remove leave community
		log.Info().Msg("register route DELETE /communities/{communityId}/members")
		r.Delete("/communities/{communityId}/members", a.routerHandler(a.leaveCommunityHandler))
		// DELETE /communities/{communityId}/members/{userId} - Remove user from community
		log.Info().Msg("register route DELETE /communities/{communityId}/members/{userId}")
		r.Delete("/communities/{communityId}/members/{userId}", a.routerHandler(a.leaveCommunityHandler))
		// GET /communities/invites - Get authenticated user's pending invites
		log.Info().Msg("register route GET /communities/invites")
		r.Get("/communities/invites", a.routerHandler(a.getUserPendingInvitesHandler))
		// PUT /communities/invites/{inviteId} - Update invite status (accept/reject)
		log.Info().Msg("register route PUT /communities/invites/{inviteId}")
		r.Put("/communities/invites/{inviteId}", a.routerHandler(a.updateInviteStatusHandler))
	})

	// Public routes
	r.Group(func(r chi.Router) {
		r.Get("/ping", func(w http.ResponseWriter, r *http.Request) {
			if _, err := w.Write([]byte(".")); err != nil {
				log.Error().Err(err).Msg("failed to write response")
			}
		})
		log.Info().Msg("register route POST /login")
		r.Post("/login", a.routerHandler(a.loginHandler))
		log.Info().Msg("register route POST /register")
		r.Post("/register", a.routerHandler(a.registerHandler))
		log.Info().Msg("register route GET /info")
		r.Get("/info", a.routerHandler(a.infoHandler))
		// GET /images/{hash}
		log.Info().Msg("register route GET /images/{hash}")
		r.Get("/images/{hash}", a.routerHandler(a.imageHandler))
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
