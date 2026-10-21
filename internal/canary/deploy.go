package canary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"text/template"
	"time"

	_ "embed"

	"github.com/nais/api/pkg/apiclient"
	"github.com/nais/api/pkg/apiclient/protoapi"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"
	"sigs.k8s.io/yaml"
)

//go:embed canary.yaml.tmpl
var canaryManifestTmpl string

// applyResponse mirrors the Nais API REST apply response.
type applyResponse struct {
	Results []struct {
		Resource        string `json:"resource"`
		EnvironmentName string `json:"environmentName"`
		Status          string `json:"status"`
		Error           string `json:"error,omitempty"`
	} `json:"results"`
}

// Deploy mints a short-lived token and applies the canary manifest to every
// configured environment.
func Deploy(ctx context.Context, cfg *Config) error {
	if err := cfg.validateDeploy(); err != nil {
		return err
	}

	manifest, err := renderManifest(time.Now())
	if err != nil {
		return fmt.Errorf("rendering canary manifest: %w", err)
	}

	token, err := mintToken(ctx, cfg)
	if err != nil {
		return err
	}

	var failed []string
	for _, env := range cfg.Environments {
		if err := applyToEnvironment(ctx, cfg, token, env, manifest); err != nil {
			log.Errorf("deploying canary to %q: %s", env, err)
			failed = append(failed, env)
			continue
		}
		log.Infof("deployed canary to %q", env)
	}

	if len(failed) != 0 {
		return fmt.Errorf("canary failed to deploy to: %s", strings.Join(failed, ", "))
	}

	return nil
}

func mintToken(ctx context.Context, cfg *Config) (string, error) {
	client, err := apiclient.New(cfg.GRPCTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return "", fmt.Errorf("connecting to nais api: %w", err)
	}
	defer func() { _ = client.Close() }()

	provision := client.ServiceAccounts()

	saResp, err := ensureServiceAccount(ctx, client)
	if err != nil {
		return "", err
	}

	tokenResp, err := provision.CreateToken(ctx, protoapi.CreateServiceAccountTokenRequest_builder{
		ServiceAccountId: saResp.GetId(),
		Name:             "deploy-canary-" + strconv.FormatInt(time.Now().Unix(), 10),
		Description:      "Short-lived token for a canary deploy run",
		ExpiresAt:        timestamppb.New(tokenExpiry(time.Now())),
	}.Build())
	if err != nil {
		return "", fmt.Errorf("minting token: %w", err)
	}

	return tokenResp.GetSecret(), nil
}

func renderManifest(now time.Time) (map[string]any, error) {
	tmpl, err := template.New("canary").Parse(canaryManifestTmpl)
	if err != nil {
		return nil, err
	}

	// DEPLOY_START is nanoseconds since epoch
	data := struct{ DeployStart string }{
		DeployStart: strconv.FormatInt(now.UnixNano(), 10),
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}

	var manifest map[string]any
	if err := yaml.Unmarshal(buf.Bytes(), &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

func applyToEnvironment(ctx context.Context, cfg *Config, token, env string, manifest map[string]any) error {
	url := fmt.Sprintf("%s/api/v1/teams/%s/environments/%s/apply", cfg.APIBaseURL, TeamSlug, env)

	payload := struct {
		Resources []map[string]any `json:"resources"`
	}{
		Resources: []map[string]any{manifest},
	}

	body := &bytes.Buffer{}
	if err := json.NewEncoder(body).Encode(payload); err != nil {
		return fmt.Errorf("encoding apply request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, body)
	if err != nil {
		return fmt.Errorf("building apply request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending apply request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("apply failed with HTTP status %d", resp.StatusCode)
	}

	var applyResp applyResponse
	if err := json.NewDecoder(resp.Body).Decode(&applyResp); err != nil {
		return fmt.Errorf("decoding apply response: %w", err)
	}

	for _, r := range applyResp.Results {
		if r.Error != "" {
			return fmt.Errorf("resource %q: %s", r.Resource, r.Error)
		}
	}

	return nil
}
