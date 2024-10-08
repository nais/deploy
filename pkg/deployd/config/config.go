package config

import (
	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	AutoCreateServiceAccount  bool   `json:"auto-create-service-account"`
	Cluster                   string `json:"cluster"`
	GRPC                      GRPC   `json:"grpc"`
	HookdKey                  string `json:"hookd-key"`
	LogFormat                 string `json:"log-format"`
	LogLevel                  string `json:"log-level"`
	MetricsListenAddr         string `json:"metrics-listen-address"`
	MetricsPath               string `json:"metrics-path"`
	OpenTelemetryCollectorURL string `json:"otel-exporter-otlp-endpoint"`
	TeamNamespaces            bool   `json:"team-namespaces"`
}

type GRPC struct {
	Authentication bool   `json:"authentication"`
	UseTLS         bool   `json:"use-tls"`
	Server         string `json:"server"`
}

const (
	Cluster                  = "cluster"
	GrpcAuthentication       = "grpc.authentication"
	GrpcServer               = "grpc.server"
	GrpcUseTLS               = "grpc.use-tls"
	HookdKey                 = "hookd-key"
	LogFormat                = "log-format"
	LogLevel                 = "log-level"
	MetricsListenAddr        = "metrics-listen-address"
	MetricsPath              = "metrics-path"
	OtelExporterOtlpEndpoint = "otel-exporter-otlp-endpoint"
)

func bindNAIS() {
	viper.BindEnv(HookdKey, "HOOKD_KEY")
	viper.BindEnv(OtelExporterOtlpEndpoint, "OTEL_EXPORTER_OTLP_ENDPOINT")
}

func Initialize() *Config {
	conftools.Initialize("deployd")
	bindNAIS()

	flag.Bool(GrpcAuthentication, false, "Use authentication on gRPC connection.")
	flag.Bool(GrpcUseTLS, false, "Use TLS when connecting to gRPC server.")
	flag.String(Cluster, "local", "Apply changes only within this cluster.")
	flag.String(GrpcServer, "127.0.0.1:9090", "gRPC server endpoint on hookd.")
	flag.String(HookdKey, "", "Pre-shared key used for hookd authentication.")
	flag.String(LogFormat, "text", "Log format, either 'json' or 'text'.")
	flag.String(LogLevel, "debug", "Logging verbosity level.")
	flag.String(MetricsListenAddr, "127.0.0.1:8081", "Serve metrics on this address.")
	flag.String(MetricsPath, "/metrics", "Serve metrics on this endpoint.")
	flag.String(OtelExporterOtlpEndpoint, "", "OpenTelemetry collector endpoint URL.")

	return &Config{}
}
