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
	"net/url"
	"os"

	"github.com/ghodss/yaml"
	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

type Config struct {
	APIKey     string
	BaseURL    string
	Cluster    string
	DryRun     bool
	Owner      string
	Ref        string
	Repository string
	Resource   []string
	Team       string
}

var cfg = DefaultConfig()

const (
	deployAPIPath = "/api/v1/deploy"
)

func DefaultConfig() Config {
	return Config{
		BaseURL:    "http://localhost:8080",
		Team:       "nobody",
		Ref:        "master",
		Owner:      "navikt",
		Repository: "deployment",
		Resource:   []string{},
		Cluster:    "local",
	}
}

func init() {
	flag.ErrHelp = fmt.Errorf("\nmkdeploy creates Github deployment request payloads, signs them, and submits them to a hookd server.\n")

	flag.StringSliceVar(&cfg.Resource, "resource", cfg.Resource, "Files with Kubernetes resources.")
	flag.StringVar(&cfg.APIKey, "apikey", cfg.APIKey, "NAIS Deploy API key.")
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "Base URL of API server.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Cluster to deploy to.")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Don't actually make a HTTP request.")
	flag.StringVar(&cfg.Owner, "owner", cfg.Owner, "Owner of git repository.")
	flag.StringVar(&cfg.Ref, "ref", cfg.Ref, "Commit hash, tag or branch.")
	flag.StringVar(&cfg.Repository, "repository", cfg.Repository, "Name of Github repository.")
	flag.StringVar(&cfg.Team, "team", cfg.Team, "Team making the deployment.")
}

func mkpayload(w io.Writer, resources json.RawMessage) error {
	req := server.DeploymentRequest{
		Resources:  resources,
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

// Wrap JSON resources in a JSON array.
func wrapResources(resources []json.RawMessage) (result json.RawMessage, err error) {
	return json.Marshal(resources)
}

func fileAsJSON(path string) (json.RawMessage, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	// Since JSON is a subset of YAML, passing JSON through this method is a no-op.
	data, err := yaml.YAMLToJSON(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", path, err)
	}

	return data, nil
}

func run() error {
	var err error

	flag.Parse()

	targetURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("wrong format of base URL: %s", err)
	}
	targetURL.Path = deployAPIPath

	resources := make([]json.RawMessage, len(cfg.Resource))

	for i, path := range cfg.Resource {
		resources[i], err = fileAsJSON(path)
		if err != nil {
			return err
		}
	}

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	allResources, err := wrapResources(resources)
	if err != nil {
		return err
	}

	err = mkpayload(buf, allResources)
	if err != nil {
		return err
	}
	bufstr := buf.String()

	decoded, err := hex.DecodeString(cfg.APIKey)
	if err != nil {
		return fmt.Errorf("HMAC key must be a hex encoded string: %s", err)
	}
	sig := mksig(buf.Bytes(), decoded)

	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)
	if err != nil {
		return fmt.Errorf("internal error creating http request: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add(server.SignatureHeader, fmt.Sprintf("%s", sig))

	log.Infof("data sent....:")
	fmt.Printf(bufstr)
	log.Infof("signature....: %s", sig)

	if cfg.DryRun {
		return nil
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	log.Infof("status.......: %s", resp.Status)
	log.Infof("data received:")

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Print(string(body))

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(1)
	}
}
