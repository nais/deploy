package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"

	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/auth"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

type Message struct {
	KafkaMessage sarama.ConsumerMessage
	Status       deployment.DeploymentStatus
	Logger       log.Entry
}

var cfg = config.DefaultConfig()

func init() {
	flag.BoolVar(&cfg.EnableGithub, "github-enabled", cfg.EnableGithub, "Enable connections to Github.")
	flag.StringVar(&cfg.WebhookSecret, "github-webhook-secret", cfg.WebhookSecret, "Github pre-shared webhook secret key.")
	flag.IntVar(&cfg.ApplicationID, "github-app-id", cfg.ApplicationID, "Github App ID.")
	flag.IntVar(&cfg.InstallID, "github-install-id", cfg.InstallID, "Github App installation ID.")
	flag.StringVar(&cfg.KeyFile, "github-key-file", cfg.KeyFile, "Path to PEM key owned by Github App.")
	flag.StringVar(&cfg.ClientID, "github-client-id", cfg.ClientID, "Client ID of the Github App.")
	flag.StringVar(&cfg.ClientSecret, "github-client-secret", cfg.ClientSecret, "Client secret of the GitHub App.")

	flag.StringVar(&cfg.ListenAddress, "listen-address", cfg.ListenAddress, "IP:PORT")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")

	flag.StringVar(&cfg.S3.Endpoint, "s3-endpoint", cfg.S3.Endpoint, "S3 endpoint for state storage.")
	flag.StringVar(&cfg.S3.AccessKey, "s3-access-key", cfg.S3.AccessKey, "S3 access key.")
	flag.StringVar(&cfg.S3.SecretKey, "s3-secret-key", cfg.S3.SecretKey, "S3 secret key.")
	flag.StringVar(&cfg.S3.BucketName, "s3-bucket-name", cfg.S3.BucketName, "S3 bucket name.")
	flag.StringVar(&cfg.S3.BucketLocation, "s3-bucket-location", cfg.S3.BucketLocation, "S3 bucket location.")
	flag.BoolVar(&cfg.S3.UseTLS, "s3-secure", cfg.S3.UseTLS, "Use TLS for S3 connections.")

	kafka.SetupFlags(&cfg.Kafka)
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	kafkaLogger, err := logging.New(cfg.Kafka.Verbosity, cfg.LogFormat)
	if err != nil {
		return err
	}

	log.Info("hookd is starting")
	log.Infof("kafka topic for requests: %s", cfg.Kafka.RequestTopic)
	log.Infof("kafka topic for statuses: %s", cfg.Kafka.StatusTopic)
	log.Infof("kafka consumer group....: %s", cfg.Kafka.GroupID)
	log.Infof("kafka brokers...........: %+v", cfg.Kafka.Brokers)
	log.Infof("web frontend templates..: %s", auth.TemplateLocation)

	sarama.Logger = kafkaLogger

	teamRepositoryStorage, err := persistence.NewS3StorageBackend(cfg.S3)
	if err != nil {
		return fmt.Errorf("while setting up S3 backend: %s", err)
	}

	kafkaClient, err := kafka.NewDualClient(
		cfg.Kafka,
		cfg.Kafka.StatusTopic,
		cfg.Kafka.RequestTopic,
	)
	if err != nil {
		return fmt.Errorf("while setting up Kafka: %s", err)
	}

	go kafkaClient.ConsumerLoop()

	var installationClient *gh.Client

	if cfg.EnableGithub {
		installationClient, err = github.InstallationClient(cfg.ApplicationID, cfg.InstallID, cfg.KeyFile)
		if err != nil {
			return fmt.Errorf("cannot instantiate Github installation client: %s", err)
		}
	}

	baseHandler := server.Handler{
		Config:                   *cfg,
		KafkaClient:              kafkaClient,
		KafkaTopic:               cfg.Kafka.RequestTopic,
		SecretToken:              cfg.WebhookSecret,
		GithubInstallationClient: installationClient,
		TeamRepositoryStorage:    teamRepositoryStorage,
	}

	deploymentHandler := &server.DeploymentHandler{Handler: baseHandler}

	http.Handle("/events", deploymentHandler)
	http.Handle("/auth/login", &auth.LoginHandler{
		ClientID: cfg.ClientID,
	})
	http.Handle("/auth/callback", &auth.CallbackHandler{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
	})
	http.Handle("/auth/form", &auth.FormHandler{})
	http.Handle("/auth/submit", &auth.SubmittedFormHandler{
		TeamRepositoryStorage: teamRepositoryStorage,
		ApplicationClient:     installationClient,
	})
	http.Handle("/proxy/teams", &auth.TeamsProxyHandler{
		ApplicationClient: installationClient,
	})
	http.Handle("/proxy/repositories", &auth.RepositoriesProxyHandler{})
	http.Handle("/assets/", http.StripPrefix(
		"/assets",
		http.FileServer(http.Dir(auth.StaticAssetsLocation)),
	))

	http.Handle("/auth/logout", &auth.LogoutHandler{})

	srv := &http.Server{
		Addr: cfg.ListenAddress,
	}

	go func() {
		err := srv.ListenAndServe()
		if err != nil {
			log.Error(err)
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	// This Kafka client loop listens for deployment statuses from deployd.
	// When they arrive, they are forwarded to the Github Deployments API.
	for {
		select {
		case m := <-kafkaClient.RecvQ:
			msg := Message{
				KafkaMessage: m,
				Logger:       kafka.ConsumerMessageLogger(&m),
			}

			msg.Logger.Trace("received incoming message")

			err := deployment.UnwrapMessage(m.Value, kafkaClient.SignatureKey, &msg.Status)
			if err != nil {
				msg.Logger.Error(err)
				kafkaClient.Consumer.MarkOffset(&m, "")
				continue
			}

			msg.Logger = *msg.Logger.WithField("delivery_id", msg.Status.GetDeliveryID())

			status, _, err := github.CreateDeploymentStatus(installationClient, &msg.Status)
			if err == nil {
				msg.Logger.Infof("created GitHub deployment status %d in repository %s", status.GetID(), status.GetRepositoryURL())
			} else {
				msg.Logger.Errorf("while sending deployment status to Github: %s", err)
			}

			kafkaClient.Consumer.MarkOffset(&msg.KafkaMessage, "")

		case <-signals:
			return nil
		}
	}
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
