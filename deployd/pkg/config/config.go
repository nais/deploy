package config

import (
	"os"

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
	EncryptionKey            string
	Kafka                    kafka.Config
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
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
		EncryptionKey:            getEnv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
	}
}
