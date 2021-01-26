package config

import (
	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Azure struct {
	ClientID            string `json:"app-client-id"`
	ClientSecret        string `json:"app-client-secret"`
	Tenant              string `json:"app-tenant-id"`
	WellKnownURL        string `json:"app-well-known-url"`
	TeamMembershipAppID string `json:"team-membership-app-id"`
	PreAuthorizedApps   string `json:"app-pre-authorized-apps"`
}

type Github struct {
	Enabled       bool   `json:"enabled"`
	ClientID      string `json:"client-id"`
	ClientSecret  string `json:"client-secret"`
	ApplicationID int    `json:"app-id"`
	InstallID     int    `json:"install-id"`
	KeyFile       string `json:"key-file"`
}

type Config struct {
	GrpcAddress           string `json:"grpc-address"`
	GrpcAuthentication    bool   `json:"grpc-authentication"`
	ListenAddress         string `json:"listen-address"`
	LogFormat             string `json:"log-format"`
	LogLevel              string `json:"log-level"`
	BaseURL               string `json:"base-url"`
	Azure                 Azure  `json:"azure"`
	Github                Github `json:"github"`
	DatabaseURL           string `json:"database-url"`
	MetricsPath           string `json:"metrics-path"`
	ProvisionKey          string `json:"provision-key"`
	DatabaseEncryptionKey string `json:"database-encryption-key"`
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.TeamMembershipAppID != "" &&
		a.WellKnownURL != ""
}

const (
	AzureClientId            = "azure.app-client-id"
	AzureClientSecret        = "azure.app-client-secret"
	AzurePreAuthorizedApps   = "azure.app-pre-authorized-apps"
	AzureTeamMembershipAppId = "azure.team-membership-app-id"
	AzureTenant              = "azure.app-tenant-id"
	AzureWellKnownUrl        = "azure.app-well-known-url"
	BaseUrl                  = "base-url"
	DatabaseEncryptionKey    = "database-encryption-key"
	DatabaseUrl              = "database-url"
	GithubAppId              = "github.app-id"
	GithubClientId           = "github.client-id"
	GithubClientSecret       = "github.client-secret"
	GithubEnabled            = "github.enabled"
	GithubInstallId          = "github.install-id"
	GithubKeyFile            = "github.key-file"
	GrpcAddress              = "grpc-address"
	GrpcAuthentication       = "grpc-authentication"
	ListenAddress            = "listen-address"
	LogFormat                = "log-format"
	LogLevel                 = "log-level"
	MetricsPath              = "metrics-path"
	ProvisionKey             = "provision-key"
)

// Bind environment variables provided by the NAIS platform
func bindNAIS() {
	viper.BindEnv(AzureClientId, "AZURE_APP_CLIENT_ID")
	viper.BindEnv(AzureClientSecret, "AZURE_APP_CLIENT_SECRET")
	viper.BindEnv(AzurePreAuthorizedApps, "AZURE_APP_PRE_AUTHORIZED_APPS")
	viper.BindEnv(AzureTenant, "AZURE_APP_TENANT_ID")
	viper.BindEnv(AzureWellKnownUrl, "AZURE_APP_WELL_KNOWN_URL")

	viper.BindEnv(DatabaseUrl, "DATABASE_URL")
}

func Initialize() *Config {
	conftools.Initialize("hookd")
	bindNAIS()

	// Provide command-line flags
	flag.Bool(GithubEnabled, false, "Enable connections to Github.")
	flag.Int(GithubAppId, 0, "Github App ID.")
	flag.Int(GithubInstallId, 0, "Github App installation ID.")
	flag.String(GithubKeyFile, "private-key.pem", "Path to PEM key owned by Github App.")
	flag.String(GithubClientId, "", "Client ID of the Github App.")
	flag.String(GithubClientSecret, "", "Client secret of the GitHub App.")

	flag.String(BaseUrl, "http://localhost:8080", "Base URL where hookd can be reached.")
	flag.String(ListenAddress, "127.0.0.1:8080", "IP:PORT")
	flag.String(LogFormat, "text", "Log format, either 'json' or 'text'.")
	flag.String(LogLevel, "debug", "Logging verbosity level.")
	flag.String(ProvisionKey, "", "Pre-shared key for /api/v1/provision endpoint.")
	flag.String(MetricsPath, "/metrics", "HTTP endpoint for exposed metrics.")

	flag.String(GrpcAddress, "127.0.0.1:9090", "Listen address of gRPC server.")
	flag.Bool(GrpcAuthentication, false, "Validate tokens on gRPC connection.")

	flag.String(DatabaseEncryptionKey, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff", "Key used to encrypt api keys at rest in PostgreSQL database.")
	flag.String(DatabaseUrl, "postgresql://postgres:root@127.0.0.1:5432/hookd", "PostgreSQL connection information.")

	flag.String(AzureClientId, "", "Azure ClientId.")
	flag.String(AzureClientSecret, "", "Azure ClientSecret")
	flag.String(AzureWellKnownUrl, "", "URL to Azure configuration.")
	flag.String(AzureTenant, "", "Azure Tenant")
	flag.String(AzureTeamMembershipAppId, "", "Application ID of canonical team list")
	flag.String(AzurePreAuthorizedApps, "", "Preauthorized Applications as Json")

	return &Config{}
}
