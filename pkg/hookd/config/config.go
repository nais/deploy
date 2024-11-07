package config

import (
	"time"

	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type GRPC struct {
	Address               string        `json:"address"`
	CliAuthentication     bool          `json:"cli-authentication"`
	DeploydAuthentication bool          `json:"deployd-authentication"`
	KeepaliveInterval     time.Duration `json:"keepalive-interval"`
}

type Config struct {
	BaseURL                   string        `json:"base-url"`
	DatabaseConnectTimeout    time.Duration `json:"database-connect-timeout"`
	DatabaseEncryptionKey     string        `json:"database-encryption-key"`
	DatabaseURL               string        `json:"database-url"`
	DeploydKeys               []string      `json:"deployd-keys"`
	FrontendKeys              []string      `json:"frontend-keys"`
	GRPC                      GRPC          `json:"grpc"`
	GoogleAllowedDomains      []string      `json:"google-allowed-domains"`
	GoogleClusterProjects     []string      `json:"google-cluster-projects"`
	ListenAddress             string        `json:"listen-address"`
	LogFormat                 string        `json:"log-format"`
	LogLevel                  string        `json:"log-level"`
	LogLinkFormatter          string        `json:"log-link-formatter"`
	MetricsPath               string        `json:"metrics-path"`
	OpenTelemetryCollectorURL string        `json:"otel-exporter-otlp-endpoint"`
	ProvisionKey              string        `json:"provision-key"`
	NaisAPIAddress            string        `json:"nais-api-address"`
	NaisAPIInsecureConnection bool          `json:"nais-api-insecure-connection"`
}

const (
	BaseUrl                   = "base-url"
	DatabaseConnectTimeout    = "database-connect-timeout"
	DatabaseEncryptionKey     = "database-encryption-key"
	DatabaseUrl               = "database-url"
	DeploydKeys               = "deployd-keys"
	FrontendKeys              = "frontend-keys"
	GoogleAllowedDomains      = "google-allowed-domains"
	GoogleClusterProjects     = "google-cluster-projects"
	GrpcAddress               = "grpc.address"
	GrpcCliAuthentication     = "grpc.cli-authentication"
	GrpcDeploydAuthentication = "grpc.deployd-authentication"
	GrpcKeepaliveInterval     = "grpc.keepalive-interval"
	ListenAddress             = "listen-address"
	LogFormat                 = "log-format"
	LogLevel                  = "log-level"
	LogLinkFormatter          = "log-link-formatter"
	MetricsPath               = "metrics-path"
	OtelExporterOtlpEndpoint  = "otel-exporter-otlp-endpoint"
	ProvisionKey              = "provision-key"
	NaisAPIAddress            = "nais-api-address"
	NaisAPIInsecureConnection = "nais-api-insecure-connection"
)

// Bind environment variables provided by the NAIS platform
func bindNAIS() {
	viper.BindEnv(DatabaseUrl, "DATABASE_URL")
	viper.BindEnv(OtelExporterOtlpEndpoint, "OTEL_EXPORTER_OTLP_ENDPOINT")

	viper.BindEnv(DeploydKeys, "DEPLOYD_KEYS")
	viper.BindEnv(FrontendKeys, "FRONTEND_KEYS")
}

func Initialize() *Config {
	conftools.Initialize("hookd")
	bindNAIS()

	// Provide command-line flags
	flag.String(BaseUrl, "http://localhost:8080", "Base URL where hookd can be reached.")
	flag.String(ListenAddress, "127.0.0.1:8080", "IP:PORT")
	flag.String(LogFormat, "text", "Log format, either 'json' or 'text'.")
	flag.String(LogLevel, "debug", "Logging verbosity level.")
	flag.String(LogLinkFormatter, "GCP", "Which format to generate deploy log links. Valid values are GCP or KIBANA")
	flag.String(ProvisionKey, "", "Pre-shared key for /api/v1/provision endpoint.")
	flag.String(MetricsPath, "/metrics", "HTTP endpoint for exposed metrics.")
	flag.String(OtelExporterOtlpEndpoint, "", "OpenTelemetry collector endpoint URL.")

	flag.String(GrpcAddress, "127.0.0.1:9090", "Listen address of gRPC server.")
	flag.Bool(GrpcDeploydAuthentication, false, "Validate tokens on gRPC connections from deployd.")
	flag.Bool(GrpcCliAuthentication, false, "Validate apikey on gRPC connections from CLI.")
	flag.Duration(GrpcKeepaliveInterval, time.Second*15, "Ping inactive clients every interval to determine if they are alive.")

	flag.String(DatabaseEncryptionKey, "00112233445566778899aabbccddeeff00112233445566778899aabbccddeeff", "Key used to encrypt api keys at rest in PostgreSQL database.")
	flag.String(DatabaseUrl, "postgresql://postgres:root@127.0.0.1:5432/hookd", "PostgreSQL connection information.")
	flag.Duration(DatabaseConnectTimeout, time.Minute*5, "How long to try the initial database connection.")

	flag.StringSlice(DeploydKeys, nil, "Pre-shared deployd keys, comma separated")
	flag.StringSlice(FrontendKeys, nil, "Pre-shared frontend keys, comma separated")

	flag.StringSlice(GoogleAllowedDomains, []string{}, "Allowed Google Domains")
	flag.StringSlice(GoogleClusterProjects, []string{}, "Mapping cluster to google project: cluster1=project1,cluster2=project2")

	flag.Bool(NaisAPIInsecureConnection, false, "Insecure connection to API server")
	flag.String(NaisAPIAddress, "localhost:3001", "NAIS API target")

	return &Config{}
}
