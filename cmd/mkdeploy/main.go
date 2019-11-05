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
	"time"

	"github.com/aymerick/raymond"
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
	Variables  string
}

type TemplateVariables map[string]interface{}

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

	flag.StringVar(&cfg.APIKey, "apikey", cfg.APIKey, "NAIS Deploy API key.")
	flag.StringVar(&cfg.BaseURL, "base-url", cfg.BaseURL, "Base URL of API server.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Cluster to deploy to.")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Don't actually make a HTTP request.")
	flag.StringVar(&cfg.Owner, "owner", cfg.Owner, "Owner of git repository.")
	flag.StringVar(&cfg.Ref, "ref", cfg.Ref, "Commit hash, tag or branch.")
	flag.StringSliceVar(&cfg.Resource, "resource", cfg.Resource, "Files with Kubernetes resources.")
	flag.StringVar(&cfg.Repository, "repository", cfg.Repository, "Name of Github repository.")
	flag.StringVar(&cfg.Team, "team", cfg.Team, "Team making the deployment.")
	flag.StringVar(&cfg.Variables, "vars", cfg.Variables, "File containing template variables.")
}

func mkpayload(w io.Writer, resources json.RawMessage) error {
	req := server.DeploymentRequest{
		Resources:  resources,
		Team:       cfg.Team,
		Cluster:    cfg.Cluster,
		Ref:        cfg.Ref,
		Owner:      cfg.Owner,
		Repository: cfg.Repository,
		Timestamp:  time.Now().Unix(),
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

func templatedFile(data []byte, ctx TemplateVariables) ([]byte, error) {
	template, err := raymond.Parse(string(data))
	if err != nil {
		return nil, fmt.Errorf("parse template file: %s", err)
	}

	output, err := template.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("execute template: %s", err)
	}

	return []byte(output), nil
}

func templateVariablesFromFile(path string) (TemplateVariables, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	vars := TemplateVariables{}
	err = yaml.Unmarshal(file, &vars)
	return vars, err
}

func fileAsJSON(path string, ctx TemplateVariables) (json.RawMessage, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	templated, err := templatedFile(file, ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", path, err)
	}

	// Since JSON is a subset of YAML, passing JSON through this method is a no-op.
	data, err := yaml.YAMLToJSON(templated)
	if err != nil {
		return nil, fmt.Errorf("%s: %s", path, err)
	}

	return data, nil
}

func run() error {
	var err error
	var templateVariables TemplateVariables

	flag.Parse()

	targetURL, err := url.Parse(cfg.BaseURL)
	if err != nil {
		return fmt.Errorf("wrong format of base URL: %s", err)
	}
	targetURL.Path = deployAPIPath

	if len(cfg.Variables) > 0 {
		templateVariables, err = templateVariablesFromFile(cfg.Variables)
		if err != nil {
			return fmt.Errorf("load template variables: %s", err)
		}
	}

	resources := make([]json.RawMessage, len(cfg.Resource))

	for i, path := range cfg.Resource {
		resources[i], err = fileAsJSON(path, templateVariables)
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

	fmt.Printf(bufstr)
	log.Debugf("signature....: %s", sig)

	if cfg.DryRun {
		return nil
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	log.Infof("status....: %s", resp.Status)

	response := &server.DeploymentResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(response)
	if err != nil {
		return fmt.Errorf("received invalid response from server: %s", err)
	}

	log.Infof("message...: %s", response.Message)
	log.Infof("logs......: %s", response.LogURL)
	if response.GithubDeployment != nil {
		log.Infof("github....: %s", response.GithubDeployment.GetURL())
	}

	if resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("deployment failed")
	}

	return nil
}

func main() {
	if err := run(); err != nil {
		log.Errorf("fatal: %s", err)
		os.Exit(1)
	}
}
