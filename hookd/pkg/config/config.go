package config

import (
	"os"
	"strconv"
	"strings"

	"github.com/navikt/deployment/common/pkg/kafka"
)

type Azure struct {
	ClientID            string `json:"clientid"`
	ClientSecret        string `json:"clientsecret"`
	Tenant              string `json:"tenant"`
	DiscoveryURL        string `json:"discoveryurl"`
	TeamMembershipAppID string `json:"teamMembershipAppID"`
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
	ListenAddress         string
	LogFormat             string
	LogLevel              string
	BaseURL               string
	Kafka                 kafka.Config
	Azure                 Azure
	Github                Github
	DatabaseURL           string
	MetricsPath           string
	Clusters              []string
	ProvisionKey          string
	EncryptionKey         string
	DatabaseEncryptionKey string
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.TeamMembershipAppID != "" &&
		a.DiscoveryURL != ""
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
		Clusters:      strings.FieldsFunc(getEnv("CLUSTERS", ""), func(r rune) bool { return r == ',' }),
		ListenAddress: getEnv("LISTEN_ADDRESS", "127.0.0.1:8080"),
		LogFormat:     getEnv("LOG_FORMAT", "text"),
		LogLevel:      getEnv("LOG_LEVEL", "debug"),
		Kafka:         kafka.DefaultConfig(),
		Azure: Azure{
			ClientID:            getEnv("AZURE_CLIENT_ID", ""),
			ClientSecret:        getEnv("AZURE_CLIENT_SECRET", ""),
			Tenant:              getEnv("AZURE_TENANT", ""),
			DiscoveryURL:        getEnv("AZURE_DISCOVERY_URL", "https://login.microsoftonline.com/common/discovery/v2.0/keys"),
			TeamMembershipAppID: getEnv("AZURE_TEAM_MEMBERSHIP_APP_ID", ""),
		},
		Github: Github{
			ApplicationID: parseInt(getEnv("GITHUB_APP_ID", "0")),
			ClientID:      getEnv("GITHUB_CLIENT_ID", ""),
			ClientSecret:  getEnv("GITHUB_CLIENT_SECRET", ""),
			Enabled:       parseBool(getEnv("GITHUB_ENABLED", "false")),
			InstallID:     parseInt(getEnv("GITHUB_INSTALL_ID", "0")),
			KeyFile:       getEnv("GITHUB_KEY_FILE", "private-key.pem"),
			WebhookSecret: getEnv("GITHUB_WEBHOOK_SECRET", ""),
		},
		DatabaseURL:           getEnv("DATABASE_URL", "postgresql://postgres:root@127.0.0.1:5432/hookd"),
		MetricsPath:           getEnv("METRICS_PATH", "/metrics"),
		ProvisionKey:          getEnv("PROVISION_KEY", ""),
		EncryptionKey:         getEnv("ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
		DatabaseEncryptionKey: getEnv("DATABASE_ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
	}
}
