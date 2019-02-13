package main

import (
	"encoding/json"
	"fmt"
	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/github"
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
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "debug",
		WebhookURL:    "https://hookd/events",
		ApplicationID: 0,
		KeyFile:       "private-key.pem",
	}
}

var config = DefaultConfig()

func (c *Config) addFlags() {
	flag.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "IP:PORT")
	flag.StringVar(&c.LogFormat, "log-format", c.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Logging verbosity level.")
	flag.StringVar(&c.WebhookURL, "webhook-url", c.WebhookURL, "Externally available URL to events endpoint.")
	flag.IntVar(&c.ApplicationID, "app-id", c.ApplicationID, "Github App ID.")
	flag.StringVar(&c.KeyFile, "key-file", c.KeyFile, "Path to PEM key owned by Github App.")
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

func deployment(w http.ResponseWriter, r *http.Request, data []byte) {
	log.Infof("Handling deployment event webhook")

	deploymentRequest := github.DeploymentRequest{}
	if err := json.Unmarshal(data, &deploymentRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	psk, err := secrets.RepositoryWebhookSecret(deploymentRequest.Repository.FullName)
	if err != nil {
		log.Errorf("could not retrieve pre-shared secret for repository '%s'", deploymentRequest.Repository.FullName)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sig := r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, data, []byte(psk))
	if err != nil {
		log.Errorf("invalid payload signature: %s", err)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "wrong secret: %s", err)
		return
	}

	fmt.Fprint(w, "deployment has been dispatched")
}

func handleAddedRepositories(installRequest gh.InstallationRepositoriesEvent) error {
	for _, repo := range installRequest.RepositoriesAdded {
		name := repo.GetFullName()
		installation := installRequest.GetInstallation()
		if installation == nil {
			return fmt.Errorf("empty installation object for %s, cannot install webhook", name)
		}
		id := int(installation.GetID())
		client, err := github.InstallationClient(config.ApplicationID, id, config.KeyFile)
		if err != nil {
			return fmt.Errorf("cannot instantiate Github client for %s: %s", name, err)
		}
		hook, err := github.CreateHook(client, *repo, config.WebhookURL)
		if err != nil {
			return err
		}
		log.Infof("created webhook in repository %s with id %d", name, hook.GetID())
	}
	return nil
}

func handleRemovedRepositories(installRequest gh.InstallationRepositoriesEvent) error {
	return nil
}

func addRemoveRepositories(w http.ResponseWriter, r *http.Request, data []byte) {
	log.Infof("Handling list of added or removed repositories")

	installRequest := gh.InstallationRepositoriesEvent{}
	if err := json.Unmarshal(data, &installRequest); err != nil {
		log.Errorf("while decoding JSON data: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	psk, err := secrets.ApplicationWebhookSecret()
	if err != nil {
		log.Errorf("could not retrieve pre-shared secret for application")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sig := r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, data, []byte(psk))
	if err != nil {
		log.Errorf("invalid payload signature: %s", err)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "wrong secret: %s", err)
		return
	}

	switch installRequest.GetAction() {
	case "added":
		err = handleAddedRepositories(installRequest)
	case "removed":
		err = handleRemovedRepositories(installRequest)
	default:
		err = fmt.Errorf("unknown installation action %s", installRequest.GetAction())
		log.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "%s", err)
		return
	}

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "internal server error")
		return
	}

	w.WriteHeader(http.StatusOK)
	msg := fmt.Sprintf("created webhooks for %d repositories", len(installRequest.RepositoriesAdded))
	fmt.Fprint(w, msg)
	log.Infof(msg)
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "deployment":
		deployment(w, r, data)
	case "installation_repositories":
		addRemoveRepositories(w, r, data)
	default:
		log.Infof("Received Github event of type '%s', ignoring", eventType)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
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

	http.HandleFunc("/register/repository", mainHandler)
	http.HandleFunc("/events", mainHandler)
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
