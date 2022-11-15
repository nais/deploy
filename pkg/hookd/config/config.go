package config

import (
	"time"

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
}

type Github struct {
	Enabled       bool   `json:"enabled"`
	ClientID      string `json:"client-id"`
	ClientSecret  string `json:"client-secret"`
	ApplicationID int    `json:"app-id"`
	InstallID     int    `json:"install-id"`
	KeyFile       string `json:"key-file"`
}

type GRPC struct {
	Address               string        `json:"address"`
	CliAuthentication     bool          `json:"cli-authentication"`
	DeploydAuthentication bool          `json:"deployd-authentication"`
	KeepaliveInterval     time.Duration `json:"keepalive-interval"`
}

type Config struct {
	Azure                  Azure         `json:"azure"`
	BaseURL                string        `json:"base-url"`
	ConsoleApiKey          string        `json:"console-api-key"`
	ConsoleUrl             string        `json:"console-url"`
	DatabaseEncryptionKey  string        `json:"database-encryption-key"`
	DatabaseURL            string        `json:"database-url"`
	DatabaseConnectTimeout time.Duration `json:"database-connect-timeout"`
	DeploydKeys            []string      `json:"deployd-keys"`
	FrontendKeys           []string      `json:"frontend-keys"`
	Github                 Github        `json:"github"`
	GoogleClientId         string        `json:"google-client-id"`
	GoogleAllowedDomains   []string      `json:"google-allowed-domains"`
	GoogleClusterProjects  []string      `json:"google-cluster-projects"`
	GRPC                   GRPC          `json:"grpc"`
	ListenAddress          string        `json:"listen-address"`
	LogFormat              string        `json:"log-format"`
	LogLevel               string        `json:"log-level"`
	MetricsPath            string        `json:"metrics-path"`
	ProvisionKey           string        `json:"provision-key"`
	PreProvisionedApiKeys  []string      `json:"pre-provisioned-api-keys"`
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.TeamMembershipAppID != "" &&
		a.WellKnownURL != ""
}

const (
	AzureClientId             = "azure.app-client-id"
	AzureClientSecret         = "azure.app-client-secret"
	AzureTeamMembershipAppId  = "azure.team-membership-app-id"
	AzureTenant               = "azure.app-tenant-id"
	AzureWellKnownUrl         = "azure.app-well-known-url"
	BaseUrl                   = "base-url"
	ConsoleApiKey             = "console-api-key"
	ConsoleUrl                = "console-url"
	DatabaseConnectTimeout    = "database-connect-timeout"
	DatabaseEncryptionKey     = "database-encryption-key"
	DatabaseUrl               = "database-url"
	DeploydKeys               = "deployd-keys"
	FrontendKeys              = "frontend-keys"
	GithubAppId               = "github.app-id"
	GithubClientId            = "github.client-id"
	GithubClientSecret        = "github.client-secret"
	GithubEnabled             = "github.enabled"
	GithubInstallId           = "github.install-id"
	GithubKeyFile             = "github.key-file"
	GoogleClientId            = "google-client-id"
	GoogleAllowedDomains      = "google-allowed-domains"
	GoogleClusterProjects     = "google-cluster-projects"
	GrpcAddress               = "grpc.address"
	GrpcCliAuthentication     = "grpc.cli-authentication"
	GrpcDeploydAuthentication = "grpc.deployd-authentication"
	GrpcKeepaliveInterval     = "grpc.keepalive-interval"
	ListenAddress             = "listen-address"
	LogFormat                 = "log-format"
	LogLevel                  = "log-level"
	MetricsPath               = "metrics-path"
	ProvisionKey              = "provision-key"
	PreProvisionedApiKeys     = "pre-provisioned-api-keys"
)

// Bind environment variables provided by the NAIS platform
func bindNAIS() {
	viper.BindEnv(AzureClientId, "AZURE_APP_CLIENT_ID")
	viper.BindEnv(AzureClientSecret, "AZURE_APP_CLIENT_SECRET")
	viper.BindEnv(AzureTenant, "AZURE_APP_TENANT_ID")
	viper.BindEnv(AzureWellKnownUrl, "AZURE_APP_WELL_KNOWN_URL")

	viper.BindEnv(DatabaseUrl, "DATABASE_URL")

	viper.BindEnv(DeploydKeys, "DEPLOYD_KEYS")
	viper.BindEnv(FrontendKeys, "FRONTEND_KEYS")

	viper.BindEnv(GoogleClientId, "GOOGLE_CLIENT_ID")
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
	flag.Bool(GrpcDeploydAuthentication, false, "Validate tokens on gRPC connections from deployd.")
	flag.Bool(GrpcCliAuthentication, false, "Validate apikey on gRPC connections from CLI.")
	flag.Duration(GrpcKeepaliveInterval, time.Second*15, "Ping inactive clients every interval to determine if they are alive.")

	flag.String(DatabaseEncryptionKey, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff", "Key used to encrypt api keys at rest in PostgreSQL database.")
	flag.String(DatabaseUrl, "postgresql://postgres:root@127.0.0.1:5432/hookd", "PostgreSQL connection information.")
	flag.Duration(DatabaseConnectTimeout, time.Minute*5, "How long to try the initial database connection.")

	flag.StringSlice(DeploydKeys, nil, "Pre-shared deployd keys, comma separated")
	flag.StringSlice(FrontendKeys, nil, "Pre-shared frontend keys, comma separated")

	flag.String(ConsoleApiKey, "", "Console Api Key")
	flag.String(ConsoleUrl, "http://localhost:3000/query", "Console URL")

	flag.String(GoogleClientId, "", "Google ClientId.")
	flag.StringSlice(GoogleAllowedDomains, []string{}, "Allowed Google Domains")
	flag.StringSlice(GoogleClusterProjects, []string{}, "Mapping cluster to google project: cluster1=project1,cluster2=project2")

	flag.String(AzureClientId, "", "Azure ClientId.")
	flag.String(AzureClientSecret, "", "Azure ClientSecret")
	flag.String(AzureWellKnownUrl, "", "URL to Azure configuration.")
	flag.String(AzureTenant, "", "Azure Tenant")
	flag.String(AzureTeamMembershipAppId, "", "Application ID of canonical team list")

	flag.StringSlice(PreProvisionedApiKeys, nil, "Pre-shared team API keys, comma separated. Each entry uses the format: team;group_id;key")

	return &Config{}
}
