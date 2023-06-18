package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/emprius/emprius-app-backend/service"

	"github.com/rs/zerolog/log"
)

func main() {
	debug := flag.Bool("debug", false, "sets log level to debug")
	port := flag.Int("port", 3333, "sets the port to listen on")
	host := flag.String("host", "0.0.0.0", "sets the host to listen on")
	secret := flag.String("secret", "", "sets the secret for JWT")
	registerAuthToken := flag.String("registerAuthToken", "", "sets the registerAuthToken new users need to provide")
	flag.Parse()

	if *secret == "" {
		sb := make([]byte, 32)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random secret")
		}
		*secret = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no secret provided, using %s", *secret)
	}
	if *registerAuthToken == "" {
		sb := make([]byte, 20)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random registerAuthToken")
		}
		*registerAuthToken = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no registerAuthToken provided, using %s", *registerAuthToken)
	}

	s, err := service.New("emprius.db", *secret, *registerAuthToken, *debug)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create service")
	}
	defer s.Close()
	s.Start(*host, *port)

	log.Info().Msg("startup complete")

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warn().Msgf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	os.Exit(0)
}
