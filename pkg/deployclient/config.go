package deployclient

import (
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nais/deploy/pkg/telemetry"
	flag "github.com/spf13/pflag"
)

type Config struct {
	APIKey                    string
	Actions                   bool
	Cluster                   string
	DeployServerURL           string
	DryRun                    bool
	Environment               string
	GithubToken               string
	GrpcAuthentication        bool
	GrpcUseTLS                bool
	OpenTelemetryCollectorURL string
	Owner                     string
	PollInterval              time.Duration
	PrintPayload              bool
	Quiet                     bool
	Ref                       string
	Repository                string
	Resource                  []string
	Retry                     bool
	RetryInterval             time.Duration
	Team                      string
	Traceparent               string
	TelemetryInput            string
	Telemetry                 *telemetry.PipelineTimings
	Timeout                   time.Duration
	TracingDashboardURL       string
	Variables                 []string
	VariablesFile             string
	Wait                      bool
	WorkloadImage             string
	WorkloadName              string
}

func InitConfig(cfg *Config) {
	flag.StringVar(&cfg.APIKey, "apikey", os.Getenv("APIKEY"), "NAIS Deploy API key. (env APIKEY)")
	flag.BoolVar(&cfg.Actions, "actions", getEnvBool("ACTIONS", false), "Use GitHub Actions compatible error and warning messages. (env ACTIONS)")
	flag.StringVar(&cfg.Cluster, "cluster", os.Getenv("CLUSTER"), "NAIS cluster to deploy into. (env CLUSTER)")
	flag.StringVar(&cfg.DeployServerURL, "deploy-server", getEnv("DEPLOY_SERVER", DefaultDeployServer), "URL to API server. (env DEPLOY_SERVER)")
	flag.BoolVar(&cfg.DryRun, "dry-run", getEnvBool("DRY_RUN", false), "Run templating, but don't actually make any requests. (env DRY_RUN)")
	flag.StringVar(&cfg.Environment, "environment", os.Getenv("ENVIRONMENT"), "Environment for GitHub deployment. Autodetected from nais.yaml if not specified. (env ENVIRONMENT)")
	flag.StringVar(&cfg.GithubToken, "github-token", os.Getenv("GITHUB_TOKEN"), "Github JWT. (env GITHUB_TOKEN)")
	flag.BoolVar(&cfg.GrpcAuthentication, "grpc-authentication", getEnvBool("GRPC_AUTHENTICATION", true), "Use team API key to authenticate requests. (env GRPC_AUTHENTICATION)")
	flag.BoolVar(&cfg.GrpcUseTLS, "grpc-use-tls", getEnvBool("GRPC_USE_TLS", true), "Use encrypted connection for gRPC calls. (env GRPC_USE_TLS)")
	flag.StringVar(&cfg.OpenTelemetryCollectorURL, "otel-collector-endpoint", getEnv("OTEL_COLLECTOR_ENDPOINT", DefaultOtelCollectorEndpoint), "OpenTelemetry collector endpoint. (env OTEL_COLLECTOR_ENDPOINT)")
	flag.StringVar(&cfg.Owner, "owner", getEnv("OWNER", DefaultOwner), "Owner of GitHub repository. (env OWNER)")
	flag.BoolVar(&cfg.PrintPayload, "print-payload", getEnvBool("PRINT_PAYLOAD", false), "Print templated resources to standard output. (env PRINT_PAYLOAD)")
	flag.BoolVar(&cfg.Quiet, "quiet", getEnvBool("QUIET", false), "Suppress printing of informational messages except errors. (env QUIET)")
	flag.StringVar(&cfg.Ref, "ref", getEnv("REF", DefaultRef), "Git commit hash, tag, or branch of the code being deployed. (env REF)")
	flag.StringVar(&cfg.Repository, "repository", os.Getenv("REPOSITORY"), "Name of GitHub repository. (env REPOSITORY)")
	flag.StringSliceVar(&cfg.Resource, "resource", getEnvStringSlice("RESOURCE"), "File with Kubernetes resource. Can be specified multiple times. (env RESOURCE)")
	flag.BoolVar(&cfg.Retry, "retry", getEnvBool("RETRY", true), "Retry deploy when encountering transient errors. (env RETRY)")
	flag.StringVar(&cfg.Team, "team", os.Getenv("TEAM"), "Team making the deployment. Auto-detected from nais.yaml if possible. (env TEAM)")
	flag.StringVar(&cfg.Traceparent, "traceparent", os.Getenv("TRACEPARENT"), "The W3C Trace Context traceparent value for the workflow run. (env TRACEPARENT)")
	flag.StringVar(&cfg.TelemetryInput, "telemetry", os.Getenv("TELEMETRY"), "Telemetry data from CI pipeline. (env TELEMETRY)")
	flag.DurationVar(&cfg.Timeout, "timeout", getEnvDuration("TIMEOUT", DefaultDeployTimeout), "Time to wait for successful deployment. (env TIMEOUT)")
	flag.StringVar(&cfg.TracingDashboardURL, "tracing-dashboard-url", getEnv("TRACING_DASHBOARD_URL", DefaultTracingDashboardURL), "Base URL to Grafana tracing dashboard onto which the trace ID can be appended (env TRACING_DASHBOARD_URL)")
	flag.StringSliceVar(&cfg.Variables, "var", getEnvStringSlice("VAR"), "Template variable in the form KEY=VALUE. Can be specified multiple times. (env VAR)")
	flag.StringVar(&cfg.VariablesFile, "vars", os.Getenv("VARS"), "File containing template variables. (env VARS)")
	flag.BoolVar(&cfg.Wait, "wait", getEnvBool("WAIT", false), "Block until deployment reaches final state (success, failure, error). (env WAIT)")
	flag.StringVar(&cfg.WorkloadImage, "workload-image", getEnv("WORKLOAD_IMAGE", ""), "Use this image in a companion Image resource. (env WORKLOAD_IMAGE)")
	flag.StringVar(&cfg.WorkloadName, "workload-name", os.Getenv("WORKLOAD_NAME"), "Name of workload if no resource specified. (env WORKLOAD_NAME)")

	flag.Parse()

	// Both owner and repository must be set in a valid request, but they are not required
	if len(cfg.Owner) == 0 || len(cfg.Repository) == 0 {
		cfg.Owner = ""
		cfg.Repository = ""
	}
}

// config return user input and default values as Config.
// Values will be resolved with the following precedence: flags > environment variables > default values.
func NewConfig() *Config {
	return &Config{
		RetryInterval: time.Second * 5,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if value, ok := os.LookupEnv(key); ok {
		duration, err := time.ParseDuration(value)
		if err == nil {
			return duration
		}
	}
	return fallback
}

func getEnvStringSlice(key string) []string {
	if value, ok := os.LookupEnv(key); ok {
		return strings.Split(value, ",")
	}

	return []string{}
}

func getEnvBool(key string, def bool) bool {
	b, err := strconv.ParseBool(os.Getenv(key))
	if err != nil {
		return def
	}

	return b
}

func (cfg *Config) Validate() error {
	if len(cfg.Resource) == 0 && len(cfg.WorkloadName) == 0 {
		return ErrResourceRequired
	}

	if len(cfg.WorkloadName) > 0 && len(cfg.WorkloadImage) == 0 {
		return ErrImageRequired
	}

	if len(cfg.Cluster) == 0 {
		return ErrClusterRequired
	}

	if len(cfg.APIKey) == 0 && len(cfg.GithubToken) == 0 {
		return ErrAuthRequired
	}

	_, err := hex.DecodeString(cfg.APIKey)
	if err != nil {
		return ErrMalformedAPIKey
	}

	cfg.Telemetry, err = telemetry.ParsePipelineTelemetry(cfg.TelemetryInput)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidTelemetryFormat, err)
	}

	return nil
}
