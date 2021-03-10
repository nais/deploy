package config

import (
	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	AutoCreateServiceAccount bool   `json:"auto-create-service-account"`
	Azure                    Azure  `json:"azure"`
	Cluster                  string `json:"cluster"`
	GRPC                     GRPC   `json:"grpc"`
	HookdApplicationID       string `json:"hookd-application-id"`
	LogFormat                string `json:"log-format"`
	LogLevel                 string `json:"log-level"`
	MetricsListenAddr        string `json:"metrics-listen-address"`
	MetricsPath              string `json:"metrics-path"`
	TeamNamespaces           bool   `json:"team-namespaces"`
}

type GRPC struct {
	Authentication bool   `json:"authentication"`
	UseTLS         bool   `json:"use-tls"`
	Server         string `json:"server"`
}

type Azure struct {
	ClientID     string `json:"app-client-id"`
	ClientSecret string `json:"app-client-secret"`
	Tenant       string `json:"app-tenant-id"`
}

const (
	LogFormat          = "log-format"
	LogLevel           = "log-level"
	Cluster            = "cluster"
	MetricsListenAddr  = "metrics-listen-address"
	GrpcAuthentication = "grpc.authentication"
	GrpcUseTLS         = "grpc.use-tls"
	GrpcServer         = "grpc.server"
	HookdApplicationID = "hookd-application-id"
	MetricsPath        = "metrics-path"
	AzureClientID      = "azure.app-client-id"
	AzureClientSecret  = "azure.app-client-secret"
	AzureTenant        = "azure.app-tenant-id"
)

func bindNAIS() {
	viper.BindEnv(AzureClientID, "AZURE_APP_CLIENT_ID")
	viper.BindEnv(AzureClientSecret, "AZURE_APP_CLIENT_SECRET")
	viper.BindEnv(AzureTenant, "AZURE_APP_TENANT_ID")
}

func Initialize() *Config {
	conftools.Initialize("deployd")
	bindNAIS()

	flag.String(LogFormat, "text", "Log format, either 'json' or 'text'.")
	flag.String(LogLevel, "debug", "Logging verbosity level.")
	flag.String(Cluster, "local", "Apply changes only within this cluster.")
	flag.String(MetricsListenAddr, "127.0.0.1:8081", "Serve metrics on this address.")
	flag.Bool(GrpcUseTLS, false, "Use TLS when connecting to gRPC server.")
	flag.String(GrpcServer, "127.0.0.1:9090", "gRPC server endpoint on hookd.")
	flag.Bool(GrpcAuthentication, false, "Use token authentication on gRPC connection.")
	flag.String(HookdApplicationID, "", "Azure application ID of hookd, used for token authentication.")
	flag.String(MetricsPath, "/metrics", "Serve metrics on this endpoint.")
	flag.String(AzureClientID, "", "Azure ClientId.")
	flag.String(AzureClientSecret, "", "Azure ClientSecret")
	flag.String(AzureTenant, "", "Azure Tenant")

	return &Config{}
}
