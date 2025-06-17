package service

import (
	"context"
	"fmt"
	"github.com/emprius/emprius-app-backend/notifications"
	"os"

	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type APIConfig struct {
	DB                 *db.Database
	JwtSecret          string
	RegisterToken      string
	MaxInviteCodes     int
	InviteCodeCooldown int
	Debug              bool
	MailService        notifications.NotificationService
}

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
// It also sets the global log level to InfoLevel or DebugLevel if Debug is true.
// The service must be started with Service.Start().
// The database must be closed with Service.Close().
func New(conf *APIConfig) (*Service, error) {
	if conf == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout}).With().Caller().Logger()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if conf.Debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
	log.Info().Msg("starting app backend")

	return &Service{
		Database:           conf.DB,
		jwtSecret:          conf.JwtSecret,
		registerToken:      conf.RegisterToken,
		maxInviteCodes:     conf.MaxInviteCodes,
		inviteCodeCooldown: conf.InviteCodeCooldown,
	}, nil
}
