package config

import (
	"github.com/mitchellh/mapstructure"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
	"sort"
	"strings"
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
	GrpcAddress           string   `json:"grpc-address"`
	GrpcAuthentication    bool     `json:"grpc-authentication"`
	ListenAddress         string   `json:"listen-address"`
	LogFormat             string   `json:"log-format"`
	LogLevel              string   `json:"log-level"`
	BaseURL               string   `json:"base-url"`
	Azure                 Azure    `json:"azure"`
	Github                Github   `json:"github"`
	DatabaseURL           string   `json:"database-url"`
	MetricsPath           string   `json:"metrics-path"`
	Clusters              []string `json:"clusters"`
	ProvisionKey          string   `json:"provision-key"`
	DatabaseEncryptionKey string   `json:"database-encryption-key"`
}

func (a *Azure) HasConfig() bool {
	return a.ClientID != "" &&
		a.ClientSecret != "" &&
		a.Tenant != "" &&
		a.TeamMembershipAppID != "" &&
		a.WellKnownURL != ""
}

const (
	GithubEnabled            = "github.enabled"
	GithubAppId              = "github.app-id"
	GithubInstallId          = "github.install-id"
	GithubKeyFile            = "github.key-file"
	GithubClientId           = "github.client-id"
	GithubClientSecret       = "github.client-secret"
	BaseUrl                  = "base-url"
	ListenAddress            = "listen-address"
	LogFormat                = "log-format"
	LogLevel                 = "log-level"
	Cluster                  = "clusters"
	ProvisionKey             = "provision-key"
	GrpcAddress              = "grpc-address"
	GrpcAuthentication       = "grpc-authentication"
	DatabaseEncryptionKey    = "database-encryption-key"
	DatabaseUrl              = "database-url"
	AzureClientId            = "azure.app-client-id"
	AzureClientSecret        = "azure.app-client-secret"
	AzureWellKnownUrl        = "azure.app-well-known-url"
	AzureTenant              = "azure.app-tenant-id"
	AzureTeamMembershipAppId = "azure.team-membership-app-id"
	AzurePreAuthorizedApps   = "azure.app-pre-authorized-apps"
)

func init() {
	// Automatically read configuration options from environment variables.
	// i.e. --proxy.address will be configurable using PROXY_ADDRESS.
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// Read configuration file from working directory and/or /etc.
	// File formats supported include JSON, TOML, YAML, HCL, envfile and Java properties config files
	viper.SetConfigName("hookd")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc")

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
	flag.StringSlice(Cluster, []string{"local"}, "Comma-separated list of valid clusters that can be deployed to.")
	flag.String(ProvisionKey, "", "Pre-shared key for /api/v1/provision endpoint.")

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

}

func decoderHook(dc *mapstructure.DecoderConfig) {
	dc.TagName = "json"
	dc.ErrorUnused = true
}

func New() (*Config, error) {
	var err error
	var cfg Config

	err = viper.ReadInConfig()
	if err != nil {
		if err.(viper.ConfigFileNotFoundError) != err {
			return nil, err
		}
	}

	flag.Parse()

	err = viper.BindPFlags(flag.CommandLine)
	if err != nil {
		return nil, err
	}

	err = viper.Unmarshal(&cfg, decoderHook)
	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// Print out all configuration options except secret stuff.
func Print(redacted []string) {
	ok := func(key string) bool {
		for _, forbiddenKey := range redacted {
			if forbiddenKey == key {
				return false
			}
		}
		return true
	}

	var keys sort.StringSlice = viper.AllKeys()

	keys.Sort()
	for _, key := range keys {
		if ok(key) {
			log.Printf("%s: %v", key, viper.Get(key))
		} else {
			log.Printf("%s: ***REDACTED***", key)
		}
	}

}
