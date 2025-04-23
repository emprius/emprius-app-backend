package service

import (
	"context"
	"fmt"
	"os"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// Service is the main service struct for the API backend.
type Service struct {
	Database           *db.Database
	API                *api.API
	jwtSecret          string
	registerToken      string
	maxInviteCodes     int
	inviteCodeCooldown int
}

// Start starts the API service.
func (s *Service) Start(host string, port int) {
	s.API = api.New(s.jwtSecret, s.registerToken, s.Database, s.maxInviteCodes, s.inviteCodeCooldown)
	s.API.Start(host, port)
	log.Info().Msgf("api service started at %s:%d", host, port)
}

// Close closes the API service database.
func (s *Service) Close() {
	if err := s.Database.Close(context.Background()); err != nil {
		log.Warn().Err(err).Msg("failed to close database")
	}
}

// New creates a new API service. It creates the database and tables if they don't exist.
// It also sets the global log level to InfoLevel or DebugLevel if debug is true.
// The service must be started with Service.Start().
// The database must be closed with Service.Close().
func New(dbPath, jwtSecret, registerToken string, maxInviteCodes, inviteCodeCooldown int, debug bool) (*Service, error) {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Info().Msg("starting app backend")

	database, err := db.New(dbPath)
	if err != nil {
		return nil, err
	}
	if err := database.CreateTables(); err != nil {
		return nil, fmt.Errorf("failed to create tables: %w", err)
	}
	return &Service{
		Database:           database,
		jwtSecret:          jwtSecret,
		registerToken:      registerToken,
		maxInviteCodes:     maxInviteCodes,
		inviteCodeCooldown: inviteCodeCooldown,
	}, nil
}
