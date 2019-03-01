package config

import (
	"fmt"
	"math/rand"
	"os"
)

type Kafka struct {
	Brokers   []string
	Topic     string
	ClientID  string
	GroupID   string
	Verbosity string
}

type Config struct {
	LogFormat string
	LogLevel  string
	Kafka     Kafka
}

func DefaultGroupName() string {
	if hostname, err := os.Hostname(); err == nil {
		return fmt.Sprintf("deployd-%s", hostname)
	}
	return fmt.Sprintf("deployd-%d", rand.Int())
}

func DefaultConfig() *Config {
	defaultGroup := DefaultGroupName()
	return &Config{
		LogFormat: "text",
		LogLevel:  "debug",
		Kafka: Kafka{
			Verbosity: "trace",
			Brokers:   []string{"localhost:9092"},
			Topic:     "deployments",
			ClientID:  defaultGroup,
			GroupID:   defaultGroup,
		},
	}
}
