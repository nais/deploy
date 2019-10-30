package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"

	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

type Config struct {
	DryRun     bool
	URL        string
	Team       string
	Ref        string
	Owner      string
	Repository string
	Payload    string
	Cluster    string
	HMACKey    string
}

var cfg = DefaultConfig()

func DefaultConfig() Config {
	return Config{
		URL:        "http://localhost:8080/api/v1/deploy",
		Team:       "nobody",
		Ref:        "master",
		Owner:      "navikt",
		Repository: "deployment",
		Payload:    "[{}]",
		Cluster:    "local",
	}
}

func init() {
	flag.ErrHelp = fmt.Errorf("\nmkdeploy creates Github deployment request payloads, signs them, and submits them to a hookd server.\n")

	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Don't actually make a HTTP request.")
	flag.StringVar(&cfg.URL, "url", cfg.URL, "URL to call.")
	flag.StringVar(&cfg.Team, "team", cfg.Team, "Team making the deployment.")
	flag.StringVar(&cfg.Ref, "ref", cfg.Ref, "Commit hash, tag or branch.")
	flag.StringVar(&cfg.Owner, "owner", cfg.Owner, "Owner of git repository.")
	flag.StringVar(&cfg.Repository, "repository", cfg.Repository, "Name of Github repository.")
	flag.StringVar(&cfg.Payload, "payload", cfg.Payload, "Deployment payload.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Cluster to deploy to.")
	flag.StringVar(&cfg.HMACKey, "hmac", cfg.HMACKey, "Webhook pre-shared key.")
}

func mkpayload(w io.Writer) error {
	req := server.DeploymentRequest{
		Resources:  json.RawMessage(cfg.Payload),
		Team:       cfg.Team,
		Cluster:    cfg.Cluster,
		Ref:        cfg.Ref,
		Owner:      cfg.Owner,
		Repository: cfg.Repository,
	}

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	return enc.Encode(req)
}

func mksig(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
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

	sig := mksig(buf.Bytes(), []byte(cfg.HMACKey))

	req, err := http.NewRequest("POST", cfg.URL, buf)
	if err != nil {
		return fmt.Errorf("error creating http request: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add(server.SignatureHeader, fmt.Sprintf("%s", sig))

	log.Infof("data sent....:")
	fmt.Printf(bufstr)

	if !cfg.DryRun {

		client := http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("error making http request: %v", err)
		}

		log.Infof("status.......: %s", resp.Status)
		log.Infof("data received:")

		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Print(string(body))
	}

	return err
}

func main() {
	if err := run(); err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(1)
	}
}
