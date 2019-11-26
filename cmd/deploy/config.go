package main

import (
	"fmt"
	"strings"

	flag "github.com/spf13/pflag"

	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

type Config struct {
	Actions         bool     `json:"actions"`
	APIKey          string   `json:"apikey"`
	DeployServerURL string   `json:"deploy-server"`
	Cluster         string   `json:"cluster"`
	PrintPayload    bool     `json:"print-payload"`
	DryRun          bool     `json:"dry-run"`
	Owner           string   `json:"owner"`
	Quiet           bool     `json:"quiet"`
	Ref             string   `json:"ref"`
	Repository      string   `json:"repository"`
	Resource        []string `json:"resource"`
	Team            string   `json:"team"`
	Variables       []string `json:"var"`
	VariablesFile   string   `json:"vars"`
	Wait            bool     `json:"wait"`
}

var help = `
deploy prepares and submits Kubernetes resources to a NAIS cluster.
`

func init() {
	flag.ErrHelp = fmt.Errorf(help)

	// Automatically read configuration options from environment variables.
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))
	viper.SetConfigName("deploy")

	// Provide command-line flags
	flag.Bool("actions", false, "Use GitHub Actions compatible error and warning messages.")
	flag.String("apikey", "", "NAIS Deploy API key.")
	flag.String("deploy-server", defaultDeployServer, "URL to API server.")
	flag.String("cluster", "", "NAIS cluster to deploy into.")
	flag.Bool("dry-run", false, "Run templating, but don't actually make any requests.")
	flag.String("owner", defaultOwner, "Owner of GitHub repository.")
	flag.Bool("print-payload", false, "Print templated resources to standard output.")
	flag.Bool("quiet", false, "Suppress printing of informational messages except errors.")
	flag.String("ref", defaultRef, "Git commit hash, tag, or branch of the code being deployed.")
	flag.StringSlice("resource", []string{}, "File with Kubernetes resource. Can be specified multiple times.")
	flag.String("repository", "", "Name of GitHub repository.")
	flag.String("team", "", "Team making the deployment. Auto-detected if possible.")
	flag.StringSlice("var", []string{}, "Template variable in the form KEY=VALUE. Can be specified multiple times.")
	flag.String("vars", "", "File containing template variables.")
	flag.Bool("wait", false, "Block until deployment reaches final state (success, failure, error).")
}

func decoderHook(dc *mapstructure.DecoderConfig) {
	dc.TagName = "json"
	dc.ErrorUnused = true
}

func configuration() (*Config, error) {
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
