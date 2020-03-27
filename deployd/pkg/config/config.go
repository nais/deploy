package config

import (
	"os"
	"strconv"

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

func parseBool(str string) bool {
	b, _ := strconv.ParseBool(str)
	return b
}

func DefaultConfig() *Config {
	return &Config{
		LogFormat:                getEnv("LOG_FORMAT", "text"),
		LogLevel:                 getEnv("LOG_LEVEL", "debug"),
		Cluster:                  getEnv("CLUSTER", "local"),
		MetricsListenAddr:        getEnv("METRICS_LISTEN_ADDRESS", "127.0.0.1:8081"),
		MetricsPath:              getEnv("METRICS_PATH", "/metrics"),
		TeamNamespaces:           parseBool(getEnv("TEAM_NAMESPACES", "false")),
		AutoCreateServiceAccount: parseBool(getEnv("AUTO_CREATE_SERVICE_ACCOUNT", "true")),
		Kafka:                    kafka.DefaultConfig(),
		EncryptionKey:            getEnv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
	}
}
