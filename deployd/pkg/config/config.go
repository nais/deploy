package config

import (
	"os"
	"strconv"
)

type Config struct {
	LogFormat                string
	LogLevel                 string
	Cluster                  string
	MetricsListenAddr        string
	GrpcAuthentication       bool
	GrpcUseTLS               bool
	GrpcServer               string
	HookdApplicationID       string
	MetricsPath              string
	TeamNamespaces           bool
	AutoCreateServiceAccount bool
	Azure                    Azure
}

type Azure struct {
	ClientID     string `json:"clientid"`
	ClientSecret string `json:"clientsecret"`
	Tenant       string `json:"tenant"`
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

func DefaultConfig() *Config {
	return &Config{
		LogFormat:                getEnv("LOG_FORMAT", "text"),
		LogLevel:                 getEnv("LOG_LEVEL", "debug"),
		Cluster:                  getEnv("CLUSTER", "local"),
		GrpcAuthentication:       parseBool(getEnv("GRPC_AUTHENTICATION", "false")),
		GrpcUseTLS:               parseBool(getEnv("GRPC_USE_TLS", "false")),
		GrpcServer:               getEnv("GRPC_SERVER", "127.0.0.1:9090"),
		HookdApplicationID:       getEnv("HOOKD_APPLICATION_ID", ""),
		MetricsListenAddr:        getEnv("METRICS_LISTEN_ADDRESS", "127.0.0.1:8081"),
		MetricsPath:              getEnv("METRICS_PATH", "/metrics"),
		TeamNamespaces:           parseBool(getEnv("TEAM_NAMESPACES", "false")),
		AutoCreateServiceAccount: parseBool(getEnv("AUTO_CREATE_SERVICE_ACCOUNT", "true")),
		Azure: Azure{
			ClientID:     getEnv("AZURE_APP_CLIENT_ID", ""),
			ClientSecret: getEnv("AZURE_APP_CLIENT_SECRET", ""),
			Tenant:       getEnv("AZURE_APP_TENANT_ID", ""),
		},
	}
}
