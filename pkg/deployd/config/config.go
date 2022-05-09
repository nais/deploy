package config

import (
	"github.com/nais/liberator/pkg/conftools"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	AutoCreateServiceAccount bool   `json:"auto-create-service-account"`
	Cluster                  string `json:"cluster"`
	GRPC                     GRPC   `json:"grpc"`
	HookdToken               string `json:"hookd-token"`
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

const (
	LogFormat          = "log-format"
	LogLevel           = "log-level"
	Cluster            = "cluster"
	MetricsListenAddr  = "metrics-listen-address"
	GrpcAuthentication = "grpc.authentication"
	GrpcUseTLS         = "grpc.use-tls"
	GrpcServer         = "grpc.server"
	HookdToken         = "hookd-token"
	MetricsPath        = "metrics-path"
)

func bindNAIS() {
	viper.BindEnv(HookdToken, "HOOKD_TOKEN")
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
	flag.String(HookdToken, "", "Token used for hookd token authentication.")
	flag.String(MetricsPath, "/metrics", "Serve metrics on this endpoint.")

	return &Config{}
}
