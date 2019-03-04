package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"net/http"
	"os"
)

type Message struct {
	KafkaMessage sarama.ConsumerMessage
	Status       deployment.DeploymentStatus
	Logger       log.Entry
}

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.ListenAddress, "listen-address", cfg.ListenAddress, "IP:PORT")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.WebhookURL, "webhook-url", cfg.WebhookURL, "Externally available URL to events endpoint.")
	flag.IntVar(&cfg.ApplicationID, "app-id", cfg.ApplicationID, "Github App ID.")
	flag.IntVar(&cfg.InstallID, "install-id", cfg.InstallID, "Github App installation ID.")
	flag.StringVar(&cfg.KeyFile, "key-file", cfg.KeyFile, "Path to PEM key owned by Github App.")
	flag.StringVar(&cfg.VaultAddress, "vault-address", cfg.VaultAddress, "Address to Vault HTTP API.")
	flag.StringVar(&cfg.VaultPath, "vault-path", cfg.VaultPath, "Base path to hookd data in Vault.")

	kafka.SetupFlags(&cfg.Kafka)
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	vaultToken := os.Getenv("VAULT_TOKEN")
	if len(vaultToken) == 0 {
		return fmt.Errorf("the VAULT_TOKEN environment variable needs to be set")
	}

	secretClient, err := secrets.New(cfg.VaultAddress, vaultToken, cfg.VaultPath)
	if err != nil {
		return fmt.Errorf("while configuring secret client: %s", err)
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
	log.Infof("vault address...........: %s", cfg.VaultAddress)
	log.Infof("vault path..............: %s", cfg.VaultPath)

	sarama.Logger = kafkaLogger

	kafka, err := kafka.NewDualClient(
		cfg.Kafka.Brokers,
		cfg.Kafka.ClientID,
		cfg.Kafka.GroupID,
		cfg.Kafka.StatusTopic,
		cfg.Kafka.RequestTopic,
	)
	if err != nil {
		return fmt.Errorf("while setting up Kafka: %s", err)
	}

	githubClient, err := github.ApplicationClient(cfg.ApplicationID, cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github installation client: %s", err)
	}

	installationClient, err := github.InstallationClient(cfg.ApplicationID, cfg.InstallID, cfg.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github installation client: %s", err)
	}

	baseHandler := server.Handler{
		Config:                   *cfg,
		SecretClient:             secretClient,
		KafkaProducer:            kafka.Producer,
		KafkaTopic:               cfg.Kafka.RequestTopic,
		GithubClient:             githubClient,
		GithubInstallationClient: installationClient,
	}
	http.Handle("/register/repository", &server.LifecycleHandler{Handler: baseHandler})
	http.Handle("/events", &server.DeploymentHandler{Handler: baseHandler})
	srv := &http.Server{
		Addr: cfg.ListenAddress,
	}

	return srv.ListenAndServe()
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
