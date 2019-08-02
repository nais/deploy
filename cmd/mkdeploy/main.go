package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	gh "github.com/google/go-github/v27/github"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"io"
	"net/http"
	"os"
)

type Config struct {
	URL         string
	Repository  string
	Payload     string
	Environment string
	Description string
	HMACKey     string
}

var cfg = DefaultConfig()

func DefaultConfig() Config {
	return Config{
		URL:         "http://localhost:8080/events",
		Repository:  "navikt/deployment",
		Payload:     "{}",
		Environment: "local",
		Description: "test deployment only done locally",
	}
}

func init() {
	flag.ErrHelp = fmt.Errorf("\nmkdeploy creates Github deployment request payloads, signs them, and submits them to a hookd server.\n")

	flag.StringVar(&cfg.Repository, "repository", cfg.Repository, "Full name of Github repository.")
	flag.StringVar(&cfg.Payload, "payload", cfg.Payload, "Deployment payload.")
	flag.StringVar(&cfg.Environment, "environment", cfg.Environment, "Environment to deploy to.")
	flag.StringVar(&cfg.Description, "description", cfg.Description, "Deployment description.")
	flag.StringVar(&cfg.HMACKey, "hmac", cfg.HMACKey, "Webhook pre-shared key.")
}

func mkpayload(w io.Writer) error {
	req := gh.DeploymentEvent{
		Deployment: &gh.Deployment{
			Description: &cfg.Description,
			Environment: &cfg.Environment,
			Payload:     json.RawMessage(cfg.Payload),
		},
		Repo: &gh.Repository{
			FullName: &cfg.Repository,
		},
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(req)
}

func mksig(data, key []byte) string {
	hasher := hmac.New(sha1.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum)
}

func run() error {
	flag.Parse()

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	err := mkpayload(buf)
	if err != nil {
		return err
	}
	bufstr := buf.String()

	key, err := hex.DecodeString(cfg.HMACKey)
	if err != nil {
		return fmt.Errorf("error decoding hmac key %q: %v", cfg.HMACKey, err)
	}

	sig := mksig(buf.Bytes(), key)

	req, err := http.NewRequest("POST", cfg.URL, buf)
	if err != nil {
		return fmt.Errorf("error creating http request: %v", err)
	}

	u, _ := uuid.NewRandom()

	req.Header.Add("content-type", "application/json")
	req.Header.Add("X-Hub-Signature", fmt.Sprintf("sha1=%s", sig))
	req.Header.Add("X-GitHub-Event", "deployment")
	req.Header.Add("X-GitHub-Delivery", u.String())

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error making http request: %v", err)
	}

	log.Infof("delivery id: %s", u.String())
	log.Infof("status.....: %s", resp.Status)
	log.Infof("data sent..:")
	log.Info(bufstr)

	return err
}

func main() {
	if err := run(); err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(1)
	}
}
