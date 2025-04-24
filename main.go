package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/emprius/emprius-app-backend/service"

	"github.com/rs/zerolog/log"
)

func main() {
	flag.Bool("debug", false, "sets log level to debug")
	flag.Int("port", 3333, "sets the port to listen on")
	flag.String("host", "0.0.0.0", "sets the host to listen on")
	flag.String("secret", "", "sets the secret for JWT")
	flag.String("mongo", "mongodb://localhost:27017", "sets the mongo URI")
	flag.String("registerAuthToken", "", "sets the registerAuthToken new users need to provide")
	flag.Int("maxInviteCodes", 3, "maximum number of invite codes a user can have")
	flag.Int("inviteCodeCooldown", 7, "cooldown period in days between invite code requests")
	flag.Parse()

	// Initialize Viper
	viper.SetEnvPrefix("EMPRIUS")
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	viper.AutomaticEnv()
	host := viper.GetString("host")
	port := viper.GetInt("port")
	secret := viper.GetString("secret")
	mongoURI := viper.GetString("mongo")
	registerAuthToken := viper.GetString("registerAuthToken")
	maxInviteCodes := viper.GetInt("maxInviteCodes")
	inviteCodeCooldown := viper.GetInt("inviteCodeCooldown")
	debug := viper.GetBool("debug")

	// if no secret is provided, generate a random one
	if secret == "" {
		sb := make([]byte, 32)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random secret")
		}
		secret = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no secret provided, using %s", secret)
	}

	if registerAuthToken == "" {
		sb := make([]byte, 20)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random registerAuthToken")
		}
		registerAuthToken = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no registerAuthToken provided, using %s", registerAuthToken)
	}

	// create service
	log.Info().Msgf("connecting to database at %s", mongoURI)
	s, err := service.New(mongoURI, secret, registerAuthToken, maxInviteCodes, inviteCodeCooldown, debug)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create service")
	}
	defer s.Close()
	s.Start(host, port)

	log.Info().Msg("startup complete")

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warn().Msgf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	os.Exit(0)
}
