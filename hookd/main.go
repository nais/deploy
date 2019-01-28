package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const GithubPreSharedKey = "BxVAH2dVbbvawyFkDD3L8JLUHzMEFQQlu9YCqNq0R7BEdragxICFJtr4jJZYBbXs"

// Config contains the server (the webhook) cert and key.
type Config struct {
	ListenAddress string
	LogFormat     string
	LogLevel      string
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "info",
	}
}

var config = DefaultConfig()

// DeploymentEvent

func (c *Config) addFlags() {
	flag.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "IP:PORT")
	flag.StringVar(&c.LogFormat, "log-format", c.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Logging verbosity level.")
}

func textFormatter() log.Formatter {
	return &log.TextFormatter{
		DisableTimestamp: false,
		FullTimestamp:    true,
	}
}

func jsonFormatter() log.Formatter {
	return &log.JSONFormatter{
		TimestampFormat: time.RFC3339Nano,
	}
}

// X-Hub-Signature: sha1=6c4f5fc2fbce53aa2011cdf1b2ab37d9dc3b6ecd
func SignatureFromHeader(header string) ([]byte, error) {
	parts := strings.Split(header, "=")
	if len(parts) != 2 {
		return nil, fmt.Errorf("wrong format for hash, expected 'sha1=hash', got '%s'", header)
	}
	if parts[0] != "sha1" {
		return nil, fmt.Errorf("expected hash type 'sha1', got '%s'", parts[0])
	}
	hexSignature, err := hex.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("error in hexadecimal format '%s': %s", parts[1], err)
	}
	return hexSignature, nil
}

func events(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		return
	}

	sigHeader := r.Header.Get("X-Hub-Signature")
	sig, err := SignatureFromHeader(sigHeader)
	if err != nil {
		log.Error(err)
		return
	}

	secret := []byte(GithubPreSharedKey)
	mac := hmac.New(sha1.New, secret)
	checkSig := mac.Sum([]byte(data))

	if bytes.Compare(sig, checkSig) != 0 {
		err := fmt.Errorf("signatures differ: expected %s, got %s", string(checkSig), string(sig))
		log.Error(err)
		return
	}

	log.Info(string(data))
	w.WriteHeader(200)
}

func run() error {
	config.addFlags()
	flag.Parse()

	switch config.LogFormat {
	case "json":
		log.SetFormatter(jsonFormatter())
	case "text":
		log.SetFormatter(textFormatter())
	default:
		return fmt.Errorf("log format '%s' is not recognized", config.LogFormat)
	}

	logLevel, err := log.ParseLevel(config.LogLevel)
	if err != nil {
		return fmt.Errorf("while setting log level: %s", err)
	}
	log.SetLevel(logLevel)

	log.Info("hookd is starting")

	http.HandleFunc("/events", events)
	server := &http.Server{
		Addr: config.ListenAddress,
	}
	return server.ListenAndServe()
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
