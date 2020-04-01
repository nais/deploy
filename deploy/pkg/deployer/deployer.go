package deployer

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
	"strings"
	"time"

	"github.com/aymerick/raymond"
	"github.com/ghodss/yaml"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/navikt/deployment/hookd/pkg/api/v1/status"
	log "github.com/sirupsen/logrus"
)

type TemplateVariables map[string]interface{}

type ActionsFormatter struct{}

type ExitCode int

const (
	DeployAPIPath       = "/api/v1/deploy"
	StatusAPIPath       = "/api/v1/status"
	DefaultPollInterval = time.Second * 5
	DefaultRef          = "master"
	DefaultOwner        = "navikt"
	DefaultDeployServer = "https://deploy.nais.io"

	ResourceRequiredMsg   = "at least one Kubernetes resource is required to make sense of the deployment"
	APIKeyRequiredMsg     = "API key required"
	MalformedURLMsg       = "wrong format of deployment server URL"
	ClusterRequiredMsg    = "cluster required; see https://doc.nais.io/clusters"
	RepositoryRequiredMsg = "repository required"
	MalformedAPIKeyMsg    = "API key must be a hex encoded string"
)

// Kept separate to avoid skewing exit codes
const (
	ExitSuccess ExitCode = iota
	ExitDeploymentFailure
	ExitDeploymentError
	ExitDeploymentInactive
	ExitNoDeployment
	ExitUnavailable
	ExitInvocationFailure
	ExitInternalError
	ExitTemplateError
)

type Deployer struct {
	Client       *http.Client
	DeployServer string
}

func (d *Deployer) Run(cfg Config) (ExitCode, error) {
	setupLogging(cfg.Actions, cfg.Quiet)

	if err := validate(cfg); err != nil {
		if !cfg.DryRun {
			return ExitInvocationFailure, err
		}

		log.Warnf("Config did not pass validation: %s", err)
	}

	var err error
	var templateVariables = make(TemplateVariables)

	targetURL, _ := url.Parse(d.DeployServer)
	targetURL.Path = DeployAPIPath

	if len(cfg.VariablesFile) > 0 {
		templateVariables, err = templateVariablesFromFile(cfg.VariablesFile)
		if err != nil {
			return ExitInvocationFailure, fmt.Errorf("load template variables: %s", err)
		}
	}

	if len(cfg.Variables) > 0 {
		templateOverrides := templateVariablesFromSlice(cfg.Variables)
		for key, val := range templateOverrides {
			if oldval, ok := templateVariables[key]; ok {
				log.Warnf("Overwriting template variable '%s'; previous value was '%v'", key, oldval)
			}
			log.Infof("Setting template variable '%s' to '%v'", key, val)
			templateVariables[key] = val
		}
	}

	resources := make([]json.RawMessage, len(cfg.Resource))

	for i, path := range cfg.Resource {
		resources[i], err = fileAsJSON(path, templateVariables)
		if err != nil {
			if cfg.PrintPayload {
				errStr := err.Error()[len(path)+2:]
				line, er := detectErrorLine(errStr)
				if er == nil {
					ctx := errorContext(string(resources[i]), line, 7)
					for _, l := range ctx {
						fmt.Println(l)
					}
				}
			}
			return ExitTemplateError, err
		}
	}

	if len(cfg.Team) == 0 {
		log.Infof("Team not explicitly specified; attempting auto-detection...")
		for i, path := range cfg.Resource {
			team := detectTeam(resources[i])
			if len(team) > 0 {
				log.Infof("Detected team '%s' in path %s", team, path)
				cfg.Team = team
				break
			}
		}

		if len(cfg.Team) == 0 {
			return ExitInvocationFailure, fmt.Errorf("no team specified, and unable to auto-detect from nais.yaml")
		}
	}

	if len(cfg.Environment) == 0 {
		log.Infof("Environment not explicitly specified; attempting auto-detection...")

		namespaces := make(map[string]interface{})
		cfg.Environment = cfg.Cluster

		for i, _ := range cfg.Resource {
			namespace := detectNamespace(resources[i])
			namespaces[namespace] = new(interface{})
		}

		if len(namespaces) == 1 {
			for namespace, _ := range namespaces {
				if len(namespace) != 0 {
					cfg.Environment = fmt.Sprintf("%s:%s", cfg.Cluster, namespace)
				}
			}
		}

		log.Infof("Detected environment '%s'", cfg.Environment)
	}

	data := make([]byte, 0)
	buf := bytes.NewBuffer(data)
	allResources, err := wrapResources(resources)

	if err != nil {
		return ExitInvocationFailure, err
	}

	err = mkpayload(buf, allResources, cfg)

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
		return ExitInvocationFailure, fmt.Errorf("%s: %s", MalformedAPIKeyMsg, err)
	}

	sig := sign(buf.Bytes(), decoded)

	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)

	if err != nil {
		return ExitInternalError, fmt.Errorf("internal error creating http request: %v", err)
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add(api_v1.SignatureHeader, fmt.Sprintf("%s", sig))
	log.Infof("Submitting deployment request to %s...", targetURL.String())
	resp, err := d.Client.Do(req)

	if err != nil {
		return ExitUnavailable, err
	}

	log.Infof("status....: %s", resp.Status)
	response := &api_v1_deploy.DeploymentResponse{}
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(response)

	if err != nil {
		return ExitUnavailable, fmt.Errorf("received invalid response from server: %s", err)
	}

	log.Infof("message...: %s", response.Message)
	log.Infof("logs......: %s", response.LogURL)

	if resp.StatusCode != http.StatusCreated {
		return ExitNoDeployment, fmt.Errorf("deployment failed: %s", response.Message)
	}

	if !cfg.Wait {
		return ExitSuccess, nil
	}

	log.Infof("Polling deployment status until it has reached its final state...")

	for {
		cont, status, err := check(response.CorrelationID, decoded, *targetURL, cfg)
		if !cont {
			return status, err
		}
		if err != nil {
			log.Error(err)
		}
		time.Sleep(cfg.PollInterval)
	}
}

func setupLogging(actions, quiet bool) {
	log.SetOutput(os.Stderr)

	if actions {
		log.SetFormatter(&ActionsFormatter{})
	} else {
		log.SetFormatter(&log.TextFormatter{
			FullTimestamp:          true,
			TimestampFormat:        time.RFC3339Nano,
			DisableLevelTruncation: true,
		})
	}

	if quiet {
		log.SetLevel(log.ErrorLevel)
	}
}

// Check if a deployment has reached a terminal state.
// The first return value is true if the state might change, false otherwise.
// Additionally, returns an error if any error occurred.
func check(deploymentID string, key []byte, targetURL url.URL, cfg Config) (bool, ExitCode, error) {
	statusReq := &api_v1_status.StatusRequest{
		DeploymentID: deploymentID,
		Team:         cfg.Team,
		Timestamp:    api_v1.Timestamp(time.Now().Unix()),
	}

	payload, err := json.Marshal(statusReq)
	if err != nil {
		return false, ExitInternalError, fmt.Errorf("unable to marshal status request: %s", err)
	}

	targetURL.Path = StatusAPIPath
	buf := bytes.NewBuffer(payload)
	req, err := http.NewRequest(http.MethodPost, targetURL.String(), buf)
	if err != nil {
		return false, ExitInternalError, fmt.Errorf("internal error creating http request: %v", err)
	}

	signature := sign(payload, key)
	req.Header.Add("content-type", "application/json")
	req.Header.Add(api_v1.SignatureHeader, signature)

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

	response := &api_v1_status.StatusResponse{}
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
	case types.GithubDeploymentState_inactive:
		return false, ExitDeploymentInactive, nil
	}

	return true, ExitSuccess, nil
}

func mkpayload(w io.Writer, resources json.RawMessage, cfg Config) error {
	req := api_v1_deploy.DeploymentRequest{
		Resources:   resources,
		Team:        cfg.Team,
		Cluster:     cfg.Cluster,
		Environment: cfg.Environment,
		Ref:         cfg.Ref,
		Owner:       cfg.Owner,
		Repository:  cfg.Repository,
		Timestamp:   time.Now().Unix(),
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

func detectNamespace(resource json.RawMessage) string {
	type namespaceMeta struct {
		Metadata struct {
			Namespace string `json:"namespace"`
		} `json:"metadata"`
	}
	buf := &namespaceMeta{}
	err := json.Unmarshal(resource, buf)

	if err != nil {
		return ""
	}

	return buf.Metadata.Namespace
}

// Wrap JSON resources in a JSON array.
func wrapResources(resources []json.RawMessage) (json.RawMessage, error) {
	return json.Marshal(resources)
}

func templatedFile(data []byte, ctx TemplateVariables) ([]byte, error) {
	if len(ctx) == 0 {
		return data, nil
	}
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

func templateVariablesFromSlice(vars []string) TemplateVariables {
	tv := TemplateVariables{}
	for _, keyval := range vars {
		tokens := strings.SplitN(keyval, "=", 2)
		switch len(tokens) {
		case 2: // KEY=VAL
			tv[tokens[0]] = tokens[1]
		case 1: // KEY
			tv[tokens[0]] = true
		default:
			continue
		}
	}

	return tv
}

func detectErrorLine(e string) (int, error) {
	var line int
	_, err := fmt.Sscanf(e, "yaml: line %d:", &line)
	return line, err
}

func errorContext(content string, line int, around int) []string {
	context := make([]string, 0)
	lines := strings.Split(content, "\n")
	format := "%03d: %s"
	for l := range lines {
		context = append(context, fmt.Sprintf(format, l+1, lines[l]))
		if l+1 == line {
			helper := "     " + strings.Repeat("^", len(lines[l])) + " <--- error near this line"
			context = append(context, helper)
		}
	}
	return context
}

func fileAsJSON(path string, ctx TemplateVariables) (json.RawMessage, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: open file: %s", path, err)
	}

	templated, err := templatedFile(file, ctx)
	if err != nil {
		errMsg := strings.ReplaceAll(err.Error(), "\n", ": ")
		return nil, fmt.Errorf("%s: %s", path, errMsg)
	}

	// Since JSON is a subset of YAML, passing JSON through this method is a no-op.
	data, err := yaml.YAMLToJSON(templated)
	if err != nil {
		return templated, fmt.Errorf("%s: %s", path, err)
	}

	return data, nil
}

func (a *ActionsFormatter) Format(e *log.Entry) ([]byte, error) {
	buf := &bytes.Buffer{}
	switch e.Level {
	case log.ErrorLevel:
		buf.WriteString("::error::")
	case log.WarnLevel:
		buf.WriteString("::warn::")
	default:
		buf.WriteString("[")
		buf.WriteString(e.Time.Format(time.RFC3339Nano))
		buf.WriteString("] ")
	}
	buf.WriteString(e.Message)
	buf.WriteRune('\n')
	return buf.Bytes(), nil
}

func validate(cfg Config) error {
	if len(cfg.Resource) == 0 {
		return fmt.Errorf(ResourceRequiredMsg)
	}

	_, err := url.Parse(cfg.DeployServerURL)
	if err != nil {
		return fmt.Errorf("%s: %s", MalformedURLMsg, err)
	}

	if len(cfg.Cluster) == 0 {
		return fmt.Errorf(ClusterRequiredMsg)
	}

	if len(cfg.APIKey) == 0 {
		return fmt.Errorf(APIKeyRequiredMsg)
	}

	if len(cfg.Repository) == 0 {
		return fmt.Errorf(RepositoryRequiredMsg)
	}

	return nil
}
