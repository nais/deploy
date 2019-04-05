package config

import (
	"os"

	"github.com/navikt/deployment/common/pkg/kafka"
)

type S3 struct {
	Endpoint       string
	AccessKey      string
	SecretKey      string
	BucketName     string
	BucketLocation string
	UseTLS         bool
}

type Config struct {
	EnableGithub  bool
	ListenAddress string
	LogFormat     string
	LogLevel      string
	BaseURL       string
	WebhookSecret string
	ApplicationID int
	InstallID     int
	KeyFile       string
	Kafka         kafka.Config
	S3            S3
	ClientID      string
	ClientSecret  string
	MetricsPath   string
}

func DefaultConfig() *Config {
	return &Config{
		EnableGithub:  true,
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "debug",
		BaseURL:       "http://localhost:8080",
		WebhookSecret: os.Getenv("GITHUB_WEBHOOK_SECRET"),
		ApplicationID: 0,
		InstallID:     0,
		KeyFile:       "private-key.pem",
		Kafka:         kafka.DefaultConfig(),
		S3: S3{
			Endpoint:       "localhost:9000",
			AccessKey:      os.Getenv("S3_ACCESS_KEY"),
			SecretKey:      os.Getenv("S3_SECRET_KEY"),
			BucketName:     "deployments.nais.io",
			BucketLocation: "",
			UseTLS:         true,
		},
		ClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		ClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		MetricsPath:  "/metrics",
	}
}
