package main

import (
	"crypto/rand"
	"fmt"
	"github.com/emprius/emprius-app-backend/api"
	"github.com/emprius/emprius-app-backend/db"
	"github.com/emprius/emprius-app-backend/notifications/mailtemplates"
	"github.com/emprius/emprius-app-backend/notifications/smtp"
	"os"
	"os/signal"
	"syscall"
	"time"

	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"

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

	flag.String("smtpServer", "", "SMTP server")
	flag.Int("smtpPort", 587, "SMTP port")
	flag.String("smtpUsername", "", "SMTP username")
	flag.String("smtpPassword", "", "SMTP password")
	flag.String("emailFromAddress", "", "Email service from address")
	flag.String("emailFromName", "Emprius", "Email service from name")

	flag.Parse()

	// Initialize Viper
	viper.SetEnvPrefix("EMPRIUS")
	if err := viper.BindPFlags(flag.CommandLine); err != nil {
		panic(err)
	}
	viper.AutomaticEnv()
	// read the configuration
	host := viper.GetString("host")
	port := viper.GetInt("port")
	secret := viper.GetString("secret")
	registerAuthToken := viper.GetString("registerAuthToken")
	maxInviteCodes := viper.GetInt("maxInviteCodes")
	inviteCodeCooldown := viper.GetInt("inviteCodeCooldown")
	debug := viper.GetBool("debug")
	// MongoDB vars
	mongoURI := viper.GetString("mongo")
	// email vars
	smtpServer := viper.GetString("smtpServer")
	smtpPort := viper.GetInt("smtpPort")
	smtpUsername := viper.GetString("smtpUsername")
	smtpPassword := viper.GetString("smtpPassword")
	emailFromAddress := viper.GetString("emailFromAddress")
	emailFromName := viper.GetString("emailFromName")

	// if no secret is provided, generate a random one
	if secret == "" {
		sb := make([]byte, 32)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random secret")
		}
		secret = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no secret provided, using %s", secret)
	}

	// Registration master token
	if registerAuthToken == "" {
		sb := make([]byte, 20)
		if _, err := rand.Read(sb); err != nil {
			log.Fatal().Err(err).Msg("failed to generate random registerAuthToken")
		}
		registerAuthToken = fmt.Sprintf("%x", sb)
		log.Warn().Msgf("no registerAuthToken provided, using %s", registerAuthToken)
	}

	// initialize the MongoDB database
	database, err := db.New(mongoURI, secret)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not create the MongoDB database: %v", err)
	}

	// init the API configuration
	apiConf := &api.APIConfig{
		DB:                 database,
		JwtSecret:          secret,
		RegisterToken:      registerAuthToken,
		MaxInviteCodes:     maxInviteCodes,
		InviteCodeCooldown: inviteCodeCooldown,
		Debug:              debug,
	}

	// overwrite the email notifications service with the SMTP service if the
	// required parameters are set and include it in the API configuration
	if smtpServer != "" && smtpUsername != "" && smtpPassword != "" {
		if emailFromAddress == "" || emailFromName == "" {
			log.Fatal().Msgf("emailFromAddress and emailFromName are required")
		}
		apiConf.MailService = new(smtp.Email)
		if err := apiConf.MailService.New(&smtp.Config{
			FromName:     emailFromName,
			FromAddress:  emailFromAddress,
			SMTPServer:   smtpServer,
			SMTPPort:     smtpPort,
			SMTPUsername: smtpUsername,
			SMTPPassword: smtpPassword,
		}); err != nil {
			log.Fatal().Err(err).Msgf("could not create the email service: %v", err)
		}

		// load email templates
		if err := mailtemplates.Load(); err != nil {
			log.Fatal().Err(err).Msgf("could not load email templates: %v", err)
		}
		log.Info().Msgf("email templates loaded: %d templates", len(mailtemplates.Available()))
	}

	// create service
	log.Info().Msgf("connecting to database at %s", mongoURI)
	a, err := api.New(apiConf)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create service")
	}
	defer a.Close()
	a.Start(host, port)

	log.Info().Msg("startup complete")

	// close if interrupt received
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
	log.Warn().Msgf("received SIGTERM, exiting at %s", time.Now().Format(time.RFC850))
	os.Exit(0)
}
