package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"github.com/navikt/deployment/hookd/pkg/github"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"io/ioutil"
	"net/http"
	"os"
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
		LogLevel:      "debug",
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

func registerRepository(w http.ResponseWriter, r *http.Request) {

}

func RepositorySecret(repository string) (string, error) {
	return GithubPreSharedKey, nil
}

func events(w http.ResponseWriter, r *http.Request) {
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType != "deployment" {
		log.Infof("Received Github event of type '%s', ignoring", eventType)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		log.Error(err)
		return
	}

	sigHeader := r.Header.Get("X-Hub-Signature")
	sig, err := github.SignatureFromHeader(sigHeader)
	if err != nil {
		log.Error(err)
		return
	}

	deploymentRequest := github.DeploymentRequest{}
	json.Unmarshal(data, &deploymentRequest)

	psk, err := RepositorySecret(deploymentRequest.Repository.FullName)
	if err != nil {
		log.Errorf("could not retrieve pre-shared secret for repository '%s'", deploymentRequest.Repository.FullName)
		return
	}

	secret := []byte(psk)
	mac := hmac.New(sha1.New, secret)
	mac.Write([]byte(data))
	checkSig := mac.Sum(nil)

	if !hmac.Equal(checkSig, sig) {
		err := fmt.Errorf("signatures differ: expected %x, got %x", checkSig, sig)
		log.Error(err)
		w.WriteHeader(403)
		fmt.Fprint(w, "wrong secret")
		return
	}

	w.WriteHeader(200)
	fmt.Fprint(w, "deployment has been dispatched")
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

	http.HandleFunc("/register/repository", registerRepository)
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
