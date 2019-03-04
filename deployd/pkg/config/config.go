package config

import (
	"github.com/navikt/deployment/common/pkg/kafka"
)

type Config struct {
	LogFormat string
	LogLevel  string
	Cluster   string
	Kafka     kafka.Config
}

func DefaultConfig() *Config {
	return &Config{
		LogFormat: "text",
		LogLevel:  "debug",
		Cluster:   "local",
		Kafka:     kafka.DefaultConfig(),
	}
}
