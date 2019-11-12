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
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/server"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

type Config struct {
	APIKey          string
	DeployServerURL string
	Cluster         string
	PrintPayload    bool
	DryRun          bool
	Owner           string
	Quiet           bool
	Ref             string
	Repository      string
	Resource        []string
	Team            string
	Variables       string
	Wait            bool
}

type TemplateVariables map[string]interface{}

var cfg = DefaultConfig()

var help = `
deploy prepares and submits Kubernetes resources to a NAIS cluster.
`

const (
	deployAPIPath       = "/api/v1/deploy"
	statusAPIPath       = "/api/v1/status"
	pollInterval        = time.Second * 5
	defaultRef          = "master"
	defaultOwner        = "navikt"
	defaultDeployServer = "https://deployment.prod-sbs.nais.io"
)

type ExitCode int

const (
	ExitSuccess ExitCode = iota
	ExitDeploymentFailure
	ExitDeploymentError
	ExitNoDeployment
	ExitUnavailable
	ExitInvocationFailure
	ExitInternalError
)

func DefaultConfig() Config {
	return Config{
		DeployServerURL: defaultDeployServer,
		Ref:             defaultRef,
		Owner:           defaultOwner,
		Resource:        []string{},
	}
}

func init() {
	flag.ErrHelp = fmt.Errorf(help)

	flag.StringVar(&cfg.APIKey, "apikey", cfg.APIKey, "NAIS Deploy API key.")
	flag.StringVar(&cfg.DeployServerURL, "deploy-server", cfg.DeployServerURL, "URL to API server.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "NAIS cluster to deploy into.")
	flag.BoolVar(&cfg.DryRun, "dry-run", cfg.DryRun, "Run templating, but don't actually make any requests.")
	flag.StringVar(&cfg.Owner, "owner", cfg.Owner, "Owner of GitHub repository.")
	flag.BoolVar(&cfg.PrintPayload, "print-payload", cfg.PrintPayload, "Print templated resources to standard output.")
	flag.BoolVar(&cfg.Quiet, "quiet", cfg.Quiet, "Suppress printing of informational messages except errors.")
	flag.StringVar(&cfg.Ref, "ref", cfg.Ref, "Git commit hash, tag, or branch of the code being deployed.")
	flag.StringSliceVar(&cfg.Resource, "resource", cfg.Resource, "File with Kubernetes resource. Can be specified multiple times.")
	flag.StringVar(&cfg.Repository, "repository", cfg.Repository, "Name of GitHub repository.")
	flag.StringVar(&cfg.Team, "team", cfg.Team, "Team making the deployment. Auto-detected if possible.")
	flag.StringVar(&cfg.Variables, "vars", cfg.Variables, "File containing template variables.")
	flag.BoolVar(&cfg.Wait, "wait", cfg.Wait, "Block until deployment reaches final state (success, failure, error).")

	flag.Parse()

	log.SetOutput(os.Stderr)
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp:          true,
		TimestampFormat:        time.RFC3339Nano,
		DisableLevelTruncation: true,
	})

	if cfg.Quiet {
		log.SetLevel(log.ErrorLevel)
	}
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

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)
	return hex.EncodeToString(sum)
}

func detectTeam(resource json.RawMessage) string {
	type teamMeta struct {
		Metadata struct {
			Labels struct {
				Team string `json:"team"`
			} `json:"labels"`
		} `json:"metadata"`
	}
	buf := &teamMeta{}
	err := json.Unmarshal(resource, buf)
	if err != nil {
		return ""
	}
	return buf.Metadata.Labels.Team
}

// Wrap JSON resources in a JSON array.
func wrapResources(resources []json.RawMessage) (json.RawMessage, error) {
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

func run() (ExitCode, error) {
	var err error
	var templateVariables TemplateVariables

	if len(cfg.Resource) == 0 {
		return ExitInvocationFailure, fmt.Errorf("at least one Kubernetes resource is required to make sense of the deployment")
	}

	targetURL, err := url.Parse(cfg.DeployServerURL)
	if err != nil {
		return ExitInvocationFailure, fmt.Errorf("wrong format of base URL: %s", err)
	}
	targetURL.Path = deployAPIPath

	if len(cfg.Variables) > 0 {
		templateVariables, err = templateVariablesFromFile(cfg.Variables)
		if err != nil {
			return ExitInvocationFailure, fmt.Errorf("load template variables: %s", err)
		}
	}

	resources := make([]json.RawMessage, len(cfg.Resource))

	for i, path := range cfg.Resource {
		resources[i], err = fileAsJSON(path, templateVariables)
		if err != nil {
			return ExitInvocationFailure, err
		}
	}

	if len(cfg.Team) == 0 {
		log.Infof("Team not explicitly specified; attempting auto-detection...")
		for i, path := range cfg.Resource {
			team := detectTeam(resources[i])
			if len(team) > 0 {
				log.Infof("Detected team '%s' in %s", team, path)
				cfg.Team = team
				break
			}
		}
	}

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)

	allResources, err := wrapResources(resources)
	if err != nil {
		return ExitInvocationFailure, err
	}

	err = mkpayload(buf, allResources)
	if err != nil {
		return ExitInvocationFailure, err
	}

	if cfg.PrintPayload {
		fmt.Printf(buf.String())
	}

	if cfg.DryRun {
		return ExitSuccess, nil
	}

	decoded, err := hex.DecodeString(cfg.APIKey)
	if err != nil {
		return ExitInvocationFailure, fmt.Errorf("API key must be a hex encoded string: %s", err)
	}
	sig := sign(buf.Bytes(), decoded)

	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)
	if err != nil {
		return ExitInternalError, fmt.Errorf("internal error creating http request: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add(server.SignatureHeader, fmt.Sprintf("%s", sig))

	log.Infof("Submitting deployment request to %s...", targetURL.String())
	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ExitUnavailable, err
	}

	log.Infof("status....: %s", resp.Status)

	response := &server.DeploymentResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(response)
	if err != nil {
		return ExitUnavailable, fmt.Errorf("received invalid response from server: %s", err)
	}

	log.Infof("message...: %s", response.Message)
	log.Infof("logs......: %s", response.LogURL)
	if response.GithubDeployment != nil {
		log.Infof("github....: %s", response.GithubDeployment.GetURL())
	}

	if resp.StatusCode != http.StatusCreated {
		return ExitNoDeployment, fmt.Errorf("deployment failed")
	}

	if !cfg.Wait {
		return ExitSuccess, nil
	}

	log.Infof("Polling deployment status until it has reached its final state...")

	for {
		cont, status, err := check(response.GithubDeployment.GetID(), decoded, *targetURL)
		if !cont {
			return status, err
		}
		if err != nil {
			log.Error(err)
		}
		time.Sleep(pollInterval)
	}
}

// Check if a deployment has reached a terminal state.
// The first return value is true if the state might change, false otherwise.
// Additionally, returns an error if any error occurred.
func check(deploymentID int64, key []byte, targetURL url.URL) (bool, ExitCode, error) {
	statusReq := &server.StatusRequest{
		DeploymentID: deploymentID,
		Team:         cfg.Team,
		Owner:        cfg.Owner,
		Repository:   cfg.Repository,
		Timestamp:    time.Now().Unix(),
	}

	payload, err := json.Marshal(statusReq)
	if err != nil {
		return false, ExitInternalError, fmt.Errorf("unable to marshal status request: %s", err)
	}

	targetURL.Path = statusAPIPath
	buf := bytes.NewBuffer(payload)
	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)
	if err != nil {
		return false, ExitInternalError, fmt.Errorf("internal error creating http request: %v", err)
	}

	signature := sign(payload, key)
	req.Header.Add("content-type", "application/json")
	req.Header.Add(server.SignatureHeader, signature)

	resp, err := http.DefaultClient.Do(req)
	if resp != nil && resp.StatusCode >= 400 && resp.StatusCode < 500 {
		return false, ExitInternalError, fmt.Errorf("bad request: %s", err)
	}
	if err != nil {
		return true, ExitInternalError, fmt.Errorf("error making request: %s", err)
	}

	if resp.StatusCode == http.StatusNoContent {
		log.Info("deployment: pending creation on GitHub")
		return true, ExitSuccess, nil
	} else if resp.StatusCode != http.StatusOK {
		log.Infof("status....: %s", resp.Status)
	}

	response := &server.StatusResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(response)
	if err != nil {
		return true, ExitInternalError, fmt.Errorf("received invalid response from server: %s", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Infof("message...: %s", response.Message)
	}

	if response.Status == nil {
		return true, ExitSuccess, nil
	}

	log.Infof("deployment: %s", *response.Status)

	status := types.GithubDeploymentState(types.GithubDeploymentState_value[*response.Status])
	switch status {
	case types.GithubDeploymentState_success:
		return false, ExitSuccess, nil
	case types.GithubDeploymentState_error:
		return false, ExitDeploymentError, nil
	case types.GithubDeploymentState_failure:
		return false, ExitDeploymentFailure, nil
	}

	return true, ExitSuccess, nil
}

func main() {
	code, err := run()
	if err != nil {
		log.Errorf("fatal: %s", err)
		if code == ExitInvocationFailure {
			flag.Usage()
		}
	}
	os.Exit(int(code))
}
