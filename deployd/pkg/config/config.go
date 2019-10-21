package config

import (
	"github.com/navikt/deployment/common/pkg/kafka"
)

type Config struct {
	LogFormat                string
	LogLevel                 string
	Cluster                  string
	MetricsListenAddr        string
	MetricsPath              string
	TeamNamespaces           bool
	AutoCreateServiceAccount bool
	Kafka                    kafka.Config
}

func DefaultConfig() *Config {
	return &Config{
		LogFormat:                "text",
		LogLevel:                 "debug",
		Cluster:                  "local",
		MetricsListenAddr:        "127.0.0.1:8081",
		MetricsPath:              "/metrics",
		TeamNamespaces:           false,
		AutoCreateServiceAccount: true,
		Kafka:                    kafka.DefaultConfig(),
	}
}
