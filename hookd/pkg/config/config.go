package config

import (
	"os"
	"strconv"
	"strings"
)

type Azure struct {
	ClientID            string `json:"clientid"`
	ClientSecret        string `json:"clientsecret"`
	Tenant              string `json:"tenant"`
	WellKnownURL        string `json:"well-known-url"`
	TeamMembershipAppID string `json:"teamMembershipAppID"`
	PreAuthorizedApps   string `json:"preAuthorizedApps"`
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
	GrpcAddress           string
	GrpcAuthentication    bool
	ListenAddress         string
	LogFormat             string
	LogLevel              string
	BaseURL               string
	Azure                 Azure
	Github                Github
	DatabaseURL           string
	MetricsPath           string
	Clusters              []string
	ProvisionKey          string
	DatabaseEncryptionKey string
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.TeamMembershipAppID != "" &&
		a.WellKnownURL != ""
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
		BaseURL:            getEnv("BASE_URL", "http://localhost:8080"),
		Clusters:           strings.FieldsFunc(getEnv("CLUSTERS", "local"), func(r rune) bool { return r == ',' }),
		ListenAddress:      getEnv("LISTEN_ADDRESS", "127.0.0.1:8080"),
		GrpcAddress:        getEnv("GRPC_LISTEN_ADDRESS", "127.0.0.1:9090"),
		GrpcAuthentication: parseBool(getEnv("GRPC_AUTHENTICATION", "false")),
		LogFormat:          getEnv("LOG_FORMAT", "text"),
		LogLevel:           getEnv("LOG_LEVEL", "debug"),
		Azure: Azure{
			ClientID:            getEnv("AZURE_APP_CLIENT_ID", ""),
			ClientSecret:        getEnv("AZURE_APP_CLIENT_SECRET", ""),
			Tenant:              getEnv("AZURE_APP_TENANT_ID", ""),
			WellKnownURL:        getEnv("AZURE_APP_WELL_KNOWN_URL", ""),
			PreAuthorizedApps:   getEnv("AZURE_APP_PRE_AUTHORIZED_APPS", ""),
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
		DatabaseEncryptionKey: getEnv("DATABASE_ENCRYPTION_KEY", "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff"),
	}
}
