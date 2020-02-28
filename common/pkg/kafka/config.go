package kafka

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"

	flag "github.com/spf13/pflag"
)

type SASL struct {
	Enabled   bool
	Handshake bool
	Username  string
	Password  string
}

type TLS struct {
	Enabled  bool
	Insecure bool
}

type Config struct {
	Brokers      []string
	RequestTopic string
	StatusTopic  string
	ClientID     string
	GroupID      string
	Verbosity    string
	SignatureKey string
	TLS          TLS
	SASL         SASL
}

func DefaultGroupName() string {
	if hostname, err := os.Hostname(); err == nil {
		return fmt.Sprintf("deployd-%s", hostname)
	}
	return fmt.Sprintf("deployd-%d", rand.Int())
}

func DefaultConfig() Config {
	defaultGroup := DefaultGroupName()
	return Config{
		Verbosity:    "trace",
		Brokers:      getEnvSlice("KAFKA_BROKERS", []string{"localhost:9092"}),
		RequestTopic: getEnv("KAFKA_REQUEST_TOPIC", "deploymentRequest"),
		StatusTopic:  getEnv("KAFKA_STATUS_TOPIC", "deploymentStatus"),
		SignatureKey: getEnv("KAFKA_HMAC_KEY", ""),
		ClientID:     getEnv("KAFKA_CLIENT_ID", defaultGroup),
		GroupID:      getEnv("KAFKA_GROUP_ID", defaultGroup),
		SASL: SASL{
			Enabled:   getEnvBool("KAFKA_SASL_ENABLED", false),
			Handshake: getEnvBool("KAFKA_SASL_HANDSHAKE", false),
			Username:  getEnv("KAFKA_SASL_USERNAME", ""),
			Password:  getEnv("KAFKA_SASL_PASSWORD", ""),
		},
	}
}

func SetupFlags(cfg *Config) {
	flag.StringSliceVar(&cfg.Brokers, "kafka-brokers", cfg.Brokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.RequestTopic, "kafka-topic-request", cfg.RequestTopic, "Kafka topic for deployment requests.")
	flag.StringVar(&cfg.StatusTopic, "kafka-topic-status", cfg.StatusTopic, "Kafka topic for deployment statuses.")
	flag.StringVar(&cfg.ClientID, "kafka-client-id", cfg.ClientID, "Kafka client ID.")
	flag.StringVar(&cfg.GroupID, "kafka-group-id", cfg.GroupID, "Kafka consumer group ID.")
	flag.StringVar(&cfg.Verbosity, "kafka-log-verbosity", cfg.Verbosity, "Log verbosity for Kafka client.")
	flag.BoolVar(&cfg.SASL.Enabled, "kafka-sasl-enabled", cfg.SASL.Enabled, "Enable SASL authentication.")
	flag.BoolVar(&cfg.SASL.Handshake, "kafka-sasl-handshake", cfg.SASL.Handshake, "Use handshake for SASL authentication.")
	flag.StringVar(&cfg.SASL.Username, "kafka-sasl-username", cfg.SASL.Username, "Username for Kafka authentication.")
	flag.StringVar(&cfg.SASL.Password, "kafka-sasl-password", cfg.SASL.Password, "Password for Kafka authentication.")
	flag.BoolVar(&cfg.TLS.Enabled, "kafka-tls-enabled", cfg.TLS.Enabled, "Use TLS for connecting to Kafka.")
	flag.BoolVar(&cfg.TLS.Insecure, "kafka-tls-insecure", cfg.TLS.Insecure, "Allow insecure Kafka TLS connections.")
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func getEnvSlice(key string, fallback []string) []string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.Split(value, ",")
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return fallback
	}
	return b
}
