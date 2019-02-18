package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"net/http"
	"os"
	"time"
)

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.ListenAddress, "listen-address", cfg.ListenAddress, "IP:PORT")
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.WebhookURL, "webhook-url", cfg.WebhookURL, "Externally available URL to events endpoint.")
	flag.IntVar(&cfg.ApplicationID, "app-id", cfg.ApplicationID, "Github App ID.")
	flag.StringVar(&cfg.KeyFile, "key-file", cfg.KeyFile, "Path to PEM key owned by Github App.")
	flag.StringVar(&cfg.VaultAddress, "vault-address", cfg.VaultAddress, "Address to Vault HTTP API.")
	flag.StringVar(&cfg.VaultPath, "vault-path", cfg.VaultPath, "Base path to hookd data in Vault.")
	flag.StringSliceVar(&cfg.KafkaBrokers, "kafka-brokers", cfg.KafkaBrokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.KafkaTopic, "kafka-topic", cfg.KafkaTopic, "Kafka topic for deployd communication.")
}

func textFormatter() log.Formatter {
	return &log.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
	}
}

func jsonFormatter() log.Formatter {
	return &log.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	}
}

func run() error {
	flag.Parse()

	switch cfg.LogFormat {
	case "json":
		log.SetFormatter(jsonFormatter())
	case "text":
		log.SetFormatter(textFormatter())
	default:
		return fmt.Errorf("log format '%s' is not recognized", cfg.LogFormat)
	}

	logLevel, err := log.ParseLevel(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("while setting log level: %s", err)
	}
	log.SetLevel(logLevel)

	vaultToken := os.Getenv("VAULT_TOKEN")
	if len(vaultToken) == 0 {
		return fmt.Errorf("the VAULT_TOKEN environment variable needs to be set")
	}

	secretClient, err := secrets.New(cfg.VaultAddress, vaultToken, cfg.VaultPath)
	if err != nil {
		return fmt.Errorf("while configuring secret client: %s", err)
	}

	log.Info("hookd is starting")

	kafka, err := sarama.NewSyncProducer(cfg.KafkaBrokers, nil)
	if err != nil {
		return fmt.Errorf("while configuring Kafka: %s", err)
	}

	baseHandler := server.Handler{
		Config:        *cfg,
		SecretClient:  secretClient,
		KafkaProducer: kafka,
		KafkaTopic:    cfg.KafkaTopic,
	}
	http.Handle("/register/repository", &server.LifecycleHandler{Handler: baseHandler})
	http.Handle("/events", &server.DeploymentHandler{Handler: baseHandler})
	server := &http.Server{
		Addr: cfg.ListenAddress,
	}
	return server.ListenAndServe()
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
