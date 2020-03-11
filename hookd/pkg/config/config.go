package config

import (
	"os"
	"strconv"

	"github.com/navikt/deployment/common/pkg/kafka"
)

type S3 struct {
	Endpoint       string `json:"endpoint"`
	AccessKey      string `json:"accesskey"`
	SecretKey      string `json:"secretkey"`
	BucketName     string `json:"bucketname"`
	BucketLocation string `json:"bucketlocation"`
	UseTLS         bool   `json:"secure"`
}

type Vault struct {
	CredentialsFile string
	Token           string
	Address         string
	Path            string
	AuthPath        string
	AuthRole        string
	KeyName         string
}

type Github struct {
	Enabled       bool
	ClientID      string
	ClientSecret  string
	WebhookSecret string
	ApplicationID int
	InstallID     int
	KeyFile       string
}

type Config struct {
	ListenAddress string
	LogFormat     string
	LogLevel      string
	BaseURL       string
	Kafka         kafka.Config
	S3            S3
	Github        Github
	Postgres      string
	MetricsPath   string
	Clusters      []string
	ProvisionKey  string
	EncryptionKey string
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
		ListenAddress: getEnv("LISTEN_ADDRESS", "127.0.0.1:8080"),
		LogFormat:     getEnv("LOG_FORMAT", "text"),
		LogLevel:      getEnv("LOG_LEVEL", "debug"),
		Kafka:         kafka.DefaultConfig(),
		Github: Github{
			ApplicationID: parseInt(getEnv("GITHUB_APP_ID", "0")),
			ClientID:      getEnv("GITHUB_CLIENT_ID", ""),
			ClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
			Enabled:       parseBool(getEnv("GITHUB_ENABLED", "false")),
			InstallID:     parseInt(getEnv("GITHUB_INSTALL_ID", "0")),
			KeyFile:       getEnv("GITHUB_KEY_FILE", "private-key.pem"),
			WebhookSecret: getEnv("GITHUB_WEBHOOK_SECRET", ""),
		},
		S3: S3{
			Endpoint:       getEnv("S3_ENDPOINT", "localhost:9000"),
			AccessKey:      getEnv("S3_ACCESS_KEY", "accesskey"),
			SecretKey:      getEnv("S3_SECRET_KEY", "secretkey"),
			BucketName:     getEnv("S3_BUCKET_NAME", "deployments.nais.io"),
			BucketLocation: getEnv("S3_BUCKET_LOCATION", ""),
			UseTLS:         parseBool(getEnv("S3_SECURE", "false")),
		},
		Postgres:      getEnv("POSTGRES_CONNECTION_STRING", "postgresql://postgres:root@127.0.0.1:5432/hookd"),
		MetricsPath:   getEnv("METRICS_PATH", "/metrics"),
		ProvisionKey:  getEnv("PROVISION_KEY", ""),
		EncryptionKey: getEnv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
	}
}
