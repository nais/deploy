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
	"strconv"
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

func handleAddedRepository(repo *gh.Repository, installation *gh.Installation) error {
	name := repo.GetFullName()

	if installation == nil {
		return fmt.Errorf("empty installation object for %s, cannot install webhook", name)
	}

	id := int(installation.GetID())
	client, err := github.InstallationClient(config.ApplicationID, id, config.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github client for %s: %s", name, err)
	}

	secret, err := secrets.RandomString(32)
	if err != nil {
		return fmt.Errorf("cannot generate random secret string: %s", err)
	}

	hook, err := github.CreateHook(client, *repo, config.WebhookURL, secret)
	if err != nil {
		return fmt.Errorf("while creating webhook: %s", err)
	}

	err = secretClient.WriteInstallationSecret(secrets.InstallationSecret{
		Repository:     name,
		WebhookID:      fmt.Sprintf("%d", hook.GetID()),
		WebhookSecret:  secret,
		InstallationID: strconv.Itoa(id),
	})

	if err != nil {
		return fmt.Errorf("while persisting repository secret: %s", err)
	}

	log.Infof("Created webhook in repository %s with id %d", name, hook.GetID())

	return nil
}

func handleRemovedRepository(repo *gh.Repository, installation *gh.Installation) error {
	name := repo.GetFullName()

	if installation == nil {
		return fmt.Errorf("empty installation object for %s, cannot remove data", name)
	}

	// At this point, we would really like to delete the webhook that the integration
	// automatically created upon registration. Unfortunately, we have already lost access.

	/*
	id := int(installation.GetID())
	client, err := github.InstallationClient(config.ApplicationID, id, config.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github client for %s: %s", name, err)
	}

	secret, err := secretClient.InstallationSecret(name)
	if err != nil {
		return fmt.Errorf("unable to retrieve pre-shared secret for repository '%s'", name)
	}

	webhookID, _ := strconv.ParseInt(secret.WebhookID, 10, 64)
	err = github.DeleteHook(client, *repo, webhookID)
	if err != nil {
		return fmt.Errorf("while deleting webhook: %s", err)
	}

	log.Infof("deleted webhook in repository %s with id %d", name, webhookID)
	*/

	err := secretClient.DeleteInstallationSecret(name)
	if err != nil {
		return fmt.Errorf("while deleting repository secret: %s", err)
	}

	return nil
}

func handleAddedRepositories(installRequest gh.InstallationRepositoriesEvent) error {
	for _, repo := range installRequest.RepositoriesAdded {
		installation := installRequest.GetInstallation()
		if err := handleAddedRepository(repo, installation); err != nil {
			return err
		}
	}
	return nil
}

func handleRemovedRepositories(installRequest gh.InstallationRepositoriesEvent) error {
	for _, repo := range installRequest.RepositoriesRemoved {
		installation := installRequest.GetInstallation()
		if err := handleRemovedRepository(repo, installation); err != nil {
			return err
		}
	}
	return nil
}

func addRemoveRepositories(w http.ResponseWriter, r *http.Request, data []byte) {
	installRequest := gh.InstallationRepositoriesEvent{}
	if err := json.Unmarshal(data, &installRequest); err != nil {
		log.Errorf("while decoding JSON data: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	psk, err := secretClient.ApplicationSecret()
	if err != nil {
		log.Errorf("unable to retrieve pre-shared secret for application: %s", err)
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

func deployment(w http.ResponseWriter, r *http.Request, data []byte) {
	log.Infof("Handling deployment event webhook")

	deploymentRequest := gh.DeploymentEvent{}
	if err := json.Unmarshal(data, &deploymentRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	repo := deploymentRequest.GetRepo()
	if repo == nil {
		log.Errorf("deployment request doesn't specify repository")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	install, err := secretClient.InstallationSecret(repo.GetFullName())
	if err != nil {
		log.Errorf("unable to retrieve pre-shared secret for repository '%s'", repo.GetFullName())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sig := r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, data, []byte(install.WebhookSecret))
	if err != nil {
		log.Errorf("invalid payload signature: %s", err)
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "wrong secret: %s", err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	log.Infof("Dispatching deployment for %s", repo.GetFullName())
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Errorf("%s %s from %s: error: %s", r.Method, r.URL.String(), r.Host, err)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")

	log.Infof("%s %s from %s type %s", r.Method, r.URL.String(), r.Host, eventType)

	switch eventType {
	case "ping":
		w.WriteHeader(http.StatusNoContent)
	case "deployment":
		deployment(w, r, data)
	case "installation_repositories":
		addRemoveRepositories(w, r, data)
	default:
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

	vaultToken := os.Getenv("VAULT_TOKEN")
	if len(vaultToken) == 0 {
		return fmt.Errorf("the VAULT_TOKEN environment variable needs to be set")
	}
	secretClient, err = secrets.New(config.VaultAddress, vaultToken, config.VaultPath)
	if err != nil {
		return fmt.Errorf("while configuring secret client: %s", err)
	}

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
