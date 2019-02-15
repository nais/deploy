package main

import (
	"fmt"
	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"io/ioutil"
	"net/http"
	"os"
	"time"
)

// Config contains the server (the webhook) cert and key.
type Config struct {
	ListenAddress string
	LogFormat     string
	LogLevel      string
	WebhookURL    string
	ApplicationID int
	KeyFile       string
	VaultAddress  string
	VaultPath     string
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "debug",
		WebhookURL:    "https://hookd/events",
		ApplicationID: 0,
		KeyFile:       "private-key.pem",
		VaultAddress:  "http://localhost:8200",
		VaultPath:     "/cubbyhole/hookd",
	}
}

type Handler struct {
	w          http.ResponseWriter
	r          *http.Request
	data       []byte
	log        *log.Entry
	eventType  string
	deliveryID string
}

var config = DefaultConfig()

var secretClient *secrets.Client

func (c *Config) addFlags() {
	flag.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "IP:PORT")
	flag.StringVar(&c.LogFormat, "log-format", c.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Logging verbosity level.")
	flag.StringVar(&c.WebhookURL, "webhook-url", c.WebhookURL, "Externally available URL to events endpoint.")
	flag.IntVar(&c.ApplicationID, "app-id", c.ApplicationID, "Github App ID.")
	flag.StringVar(&c.KeyFile, "key-file", c.KeyFile, "Path to PEM key owned by Github App.")
	flag.StringVar(&c.VaultAddress, "vault-address", c.VaultAddress, "Address to Vault HTTP API.")
	flag.StringVar(&c.VaultPath, "vault-path", c.VaultPath, "Base path to hookd data in Vault.")
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

func (h *Handler) prepare(w http.ResponseWriter, r *http.Request, unserialize func() error, secretToken func() (string, error)) error {
	var err error

	h.deliveryID = r.Header.Get("X-GitHub-Delivery")
	h.eventType = r.Header.Get("X-GitHub-Event")
	h.w = w
	h.r = r

	h.log = log.WithFields(log.Fields{
		"delivery_id": h.deliveryID,
		"event_type":  h.eventType,
	})

	h.log.Infof("%s %s %s", r.Method, r.RequestURI, h.eventType)

	h.data, err = ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	if h.eventType == "ping" {
		w.WriteHeader(http.StatusNoContent)
		return fmt.Errorf("received ping request")
	}

	err = unserialize()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	psk, err := secretToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	sig := h.r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, h.data, []byte(psk))
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("invalid payload signature: %s", err)
	}

	return nil
}

func (h *Handler) finish(statusCode int, err error) {
	if err != nil {
		h.log.Errorf("%s", err)
	}

	h.w.WriteHeader(statusCode)

	h.log.Infof("Finished handling request")
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

	vaultToken := os.Getenv("VAULT_TOKEN")
	if len(vaultToken) == 0 {
		return fmt.Errorf("the VAULT_TOKEN environment variable needs to be set")
	}
	secretClient, err = secrets.New(config.VaultAddress, vaultToken, config.VaultPath)
	if err != nil {
		return fmt.Errorf("while configuring secret client: %s", err)
	}

	log.Info("hookd is starting")

	http.Handle("/register/repository", &LifecycleHandler{})
	http.Handle("/events", &DeploymentHandler{})
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
