package config

import (
	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	LogFormat                string `json:"log-format"`
	LogLevel                 string `json:"log-level"`
	Cluster                  string `json:"cluster"`
	MetricsListenAddr        string `json:"metrics-listen-address"`
	GrpcAuthentication       bool   `json:"grpc-authentication"`
	GrpcUseTLS               bool   `json:"grpc-use-tls"`
	GrpcServer               string `json:"grpc-server"`
	HookdApplicationID       string `json:"hookd-application-id"`
	MetricsPath              string `json:"metrics-path"`
	TeamNamespaces           bool   `json:"team-namespaces"`
	AutoCreateServiceAccount bool   `json:"auto-create-service-account"`
	Azure                    Azure  `json:"azure"`
}

type Azure struct {
	ClientID     string `json:"app-client-id"`
	ClientSecret string `json:"app-client-secret"`
	Tenant       string `json:"app-tenant-id"`
}

const (
	LogFormat                = "log-format"
	LogLevel                 = "log-level"
	Cluster                  = "cluster"
	MetricsListenAddr        = "metrics-listen-address"
	GrpcAuthentication       = "grpc-authentication"
	GrpcUseTLS               = "grpc-use-tls"
	GrpcServer               = "grpc-server"
	HookdApplicationID       = "hookd-application-id"
	MetricsPath              = "metrics-path"
	TeamNamespaces           = "team-namespaces"
	AutoCreateServiceAccount = "auto-create-service-account"
	AzureClientID            = "azure.app-client-id"
	AzureClientSecret        = "azure.app-client-secret"
	AzureTenant              = "azure.app-tenant-id"
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
	flag.Bool(GrpcUseTLS, false, "Use secure connection when connecting to gRPC server.")
	flag.String(GrpcServer, "127.0.0.1:9090", "gRPC server endpoint on hookd.")
	flag.Bool(GrpcAuthentication, false, "Use token authentication on gRPC connection.")
	flag.String(HookdApplicationID, "", "Azure application ID of hookd, used for token authentication.")
	flag.String(MetricsPath, "/metrics", "Serve metrics on this endpoint.")
	flag.Bool(TeamNamespaces, false, "Set to true if team service accounts live in team's own namespace.")
	flag.Bool(AutoCreateServiceAccount, true, "Set to true to automatically create service accounts.")
	flag.String(AzureClientID, "", "Azure ClientId.")
	flag.String(AzureClientSecret, "", "Azure ClientSecret")
	flag.String(AzureTenant, "", "Azure Tenant")

	return &Config{}
}
