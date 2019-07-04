package config

import (
	"os"
	"strconv"

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

func parseInt(str string) int {
	i, _ := strconv.Atoi(str)
	return i
}

func DefaultConfig() *Config {
	return &Config{
		BaseURL:       getEnv("BASE_URL", "http://localhost:8080"),
		ApplicationID: parseInt(getEnv("GITHUB_APP_ID", "0")),
		ClientID:      getEnv("GITHUB_CLIENT_ID", ""),
		ClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
		EnableGithub:  parseBool(getEnv("GITHUB_ENABLED", "false")),
		InstallID:     parseInt(getEnv("GITHUB_INSTALL_ID", "0")),
		KeyFile:       getEnv("GITHUB_KEY_FILE", "private-key.pem"),
		WebhookSecret: getEnv("GITHUB_WEBHOOK_SECRET", ""),
		ListenAddress: getEnv("LISTEN_ADDRESS", ":8080"),
		LogFormat:     getEnv("LOG_FORMAT", "text"),
		LogLevel:      getEnv("LOG_LEVEL", "debug"),
		Kafka:         kafka.DefaultConfig(),
		S3: S3{
			Endpoint:       getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKey:      getEnv("S3_ACCESS_KEY", "accesskey"),
			SecretKey:      getEnv("S3_SECRET_KEY", "secretkey"),
			BucketName:     getEnv("S3_BUCKET_NAME", "deployments.nais.io"),
			BucketLocation: getEnv("S3_BUCKET_LOCATION", ""),
			UseTLS:         parseBool(getEnv("S3_SECURE", "false")),
		},
		MetricsPath: getEnv("METRICS_PATH", "/metrics"),
	}
}
