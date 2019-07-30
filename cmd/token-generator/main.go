package main

import (
	"fmt"
	"os"
	"os/signal"
	"sort"
	"strings"

	"github.com/mitchellh/mapstructure"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Github struct {
	Appid   int    `json:"appid"`
	Keyfile string `json:"keyfile"`
}

type Log struct {
	Format string `json:"format"`
	Level  string `json:"level"`
}

type CircleCI struct {
	Apitoken string `json:"apitoken"`
}

type Config struct {
	Bind     string    `json:"bind"`
	Url      string    `json:"url"`
	S3       config.S3 `json:"s3"`
	Log      Log       `json:"log"`
	Github   Github    `json:"github"`
	CircleCI CircleCI  `json:"circleci"`
}

var (
	cfg        = Config{}
	redactKeys = []string{"s3.accesskey", "s3.secretkey"}
)

func init() {
	// Automatically read configuration options from environment variables.
	// i.e. --github.applicationID will be configurable using TG_GITHUB_APPLICATION_ID.
	viper.SetEnvPrefix("TG")
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_", ".", "_"))

	// Read configuration file from working directory and/or /etc.
	// File formats supported include JSON, TOML, YAML, HCL, envfile and Java properties config files
	viper.SetConfigName("token-generator")
	viper.AddConfigPath(".")
	viper.AddConfigPath("/etc")

	// Provide command-line flags
	flag.String("url", "http://localhost:8080", "Base URL where token-generator is accessible from the end user's browser.")
	flag.String("bind", "127.0.0.1:8080", "IP:PORT to bind the listening socket to.")
	flag.Int("github.appid", 0, "Github Application ID.")
	flag.String("github.keyfile", "", "Path to PEM key owned by Github App.")
	flag.String("circleci.apitoken", "", "API token for authenticating with CircleCI.")
	flag.String("log.format", "text", "Log format, either 'json' or 'text'.")
	flag.String("log.level", "trace", "Logging verbosity level.")
	flag.String("s3.endpoint", "localhost:9000", "S3 endpoint for state storage.")
	flag.String("s3.accesskey", "accesskey", "S3 access key.")
	flag.String("s3.secretkey", "secretkey", "S3 secret key.")
	flag.String("s3.bucketname", "deployments.nais.io/v2", "S3 bucket name.")
	flag.String("s3.bucketlocation", "", "S3 bucket location.")
	flag.Bool("s3.secure", false, "Use TLS for S3 connections.")
}

// Print out all configuration options except secret stuff.
func printConfig(redacted []string) {
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
			log.Printf("%s: %s", key, viper.GetString(key))
		} else {
			log.Printf("%s: ***REDACTED***", key)
		}
	}

}

func decoderHook(dc *mapstructure.DecoderConfig) {
	dc.TagName = "json"
	dc.ErrorUnused = true
}

func run() error {
	var err error
	var noConfigFile bool

	err = viper.ReadInConfig()
	if err != nil {
		if err.(viper.ConfigFileNotFoundError) != err {
			return err
		}
		noConfigFile = true
	}

	flag.Parse()

	err = viper.BindPFlags(flag.CommandLine)
	if err != nil {
		return err
	}

	err = viper.Unmarshal(&cfg, decoderHook)
	if err != nil {
		return err
	}

	if err = logging.Setup(cfg.Log.Level, cfg.Log.Format); err != nil {
		return err
	}

	if noConfigFile {
		log.Warn("Configuration file not found")
	}
	printConfig(redactKeys)

	_, err = persistence.NewS3StorageBackend(cfg.S3)
	if err != nil {
		return fmt.Errorf("while setting up S3 backend: %s", err)
	}

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	for {
		select {
		case <-signals:
			return nil
		}
	}
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
