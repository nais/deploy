package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
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
}

func DefaultConfig() *Config {
	return &Config{
		ListenAddress: ":8080",
		LogFormat:     "text",
		LogLevel:      "debug",
		WebhookURL:    "https://hookd/events",
	}
}

var config = DefaultConfig()

// DeploymentEvent

func (c *Config) addFlags() {
	flag.StringVar(&c.ListenAddress, "listen-address", c.ListenAddress, "IP:PORT")
	flag.StringVar(&c.LogFormat, "log-format", c.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&c.LogLevel, "log-level", c.LogLevel, "Logging verbosity level.")
	flag.StringVar(&c.WebhookURL, "webhook-url", c.LogLevel, "Externally available URL to events endpoint.")
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

func sign(psk string, data []byte) []byte {
	secret := []byte(psk)
	mac := hmac.New(sha1.New, secret)
	mac.Write([]byte(data))
	return mac.Sum(nil)
}

func comparehmac(checkSig, sig []byte) error {
	if hmac.Equal(checkSig, sig) {
		return nil
	}
	return fmt.Errorf("signatures differ: expected %x, got %x", checkSig, sig)
}

func deployment(w http.ResponseWriter, r *http.Request, data, sig []byte) {
	deploymentRequest := github.DeploymentRequest{}
	if err := json.Unmarshal(data, &deploymentRequest); err != nil {
		w.WriteHeader(400)
		return
	}

	psk, err := secrets.RepositoryWebhookSecret(deploymentRequest.Repository.FullName)
	if err != nil {
		log.Errorf("could not retrieve pre-shared secret for repository '%s'", deploymentRequest.Repository.FullName)
		w.WriteHeader(500)
		return
	}

	checkSig := sign(psk, data)

	if comparehmac(checkSig, sig) != nil {
		log.Error(err)
		w.WriteHeader(403)
		fmt.Fprint(w, "wrong secret")
		return
	}

	fmt.Fprint(w, "deployment has been dispatched")
}

func CreateHook(r github.Repository) (*github.Webhook, error) {
	// https://developer.github.com/v3/repos/hooks/#create-a-hook
	secret, err := secrets.RandomString(32)
	if err != nil {
		return nil, err
	}

	webhook := github.Webhook{
		Name: "web",
		Events: []string{
			"deployment",
		},
		Active: true,
		Config: github.WebhookConfig{
			Url:         config.WebhookURL,
			ContentType: "json",
			InsecureSSL: "0",
			Secret:      secret,
		},
	}

	b, err := json.Marshal(webhook)
	if err != nil {
		return nil, fmt.Errorf("while marshalling webhook to JSON: %s", err)
	}
	reader := bytes.NewReader(b)

	url := fmt.Sprintf("/repos/%s/hooks", r.FullName)
	c := http.Client{}
	resp, err := c.Post(url, "application/json", reader)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("webhook creation returned status code %d, expected %d", resp.StatusCode, http.StatusCreated)
	}

	dec := json.NewDecoder(resp.Body)
	err = dec.Decode(r)
	if err != nil {
		return nil, fmt.Errorf("while decoding server response: %s", err)
	}

	log.Infof("oops, webhook secret for %s is %s", r.FullName, webhook.Config.Secret)
	return &webhook, nil
}

func registerRepository(w http.ResponseWriter, r *http.Request, data, sig []byte) {
	installRequest := github.IntegrationInstallation{}
	if err := json.Unmarshal(data, &installRequest); err != nil {
		w.WriteHeader(400)
		return
	}

	psk, err := secrets.ApplicationWebhookSecret()
	if err != nil {
		log.Errorf("could not retrieve pre-shared secret for application")
		w.WriteHeader(500)
		return
	}

	checkSig := sign(psk, data)

	if comparehmac(checkSig, sig) != nil {
		log.Error(err)
		w.WriteHeader(403)
		fmt.Fprint(w, "wrong secret")
		return
	}

	for _, repo := range installRequest.Repositories {
		_, err := CreateHook(repo)
		if err != nil {
			w.WriteHeader(500)
			fmt.Fprintf(w, "%s", err)
		}
		// TODO: write to vault
	}

	w.WriteHeader(200)
	fmt.Fprintf(w, "created webhooks for %d repositories", len(installRequest.Repositories))
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	sigHeader := r.Header.Get("X-Hub-Signature")
	sig, err := github.SignatureFromHeader(sigHeader)
	if err != nil {
		log.Error(err)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	switch eventType {
	case "deployment":
		deployment(w, r, data, sig)
	default:
		log.Infof("Received Github event of type '%s', ignoring", eventType)
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
