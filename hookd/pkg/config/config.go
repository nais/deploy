package config

import (
	"github.com/navikt/deployment/common/pkg/kafka"
)

type Config struct {
	ListenAddress string
	LogFormat     string
	LogLevel      string
	WebhookURL    string
	ApplicationID int
	InstallID     int
	KeyFile       string
	Kafka         kafka.Config
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "debug",
		WebhookURL:    "https://hookd/events",
		ApplicationID: 0,
		InstallID:     0,
		KeyFile:       "private-key.pem",
		Kafka:         kafka.DefaultConfig(),
	}
}
