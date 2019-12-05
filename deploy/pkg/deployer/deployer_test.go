package deployer_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navikt/deployment/deploy/pkg/deployer"
	apiv1deploy "github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/stretchr/testify/assert"
)

func TestHappyPath(t *testing.T) {
	cfg := validConfig()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)

		deployRequest := apiv1deploy.DeploymentRequest{}

		if err := json.NewDecoder(r.Body).Decode(&deployRequest); err != nil {
			t.Error(err)
		}

		assert.Equal(t, deployRequest.Team, "aura", "auto-detection of team works")
		assert.Equal(t, deployRequest.Owner, deployer.DefaultOwner, "defaulting works")

		b, err := json.Marshal(&apiv1deploy.DeploymentResponse{})

		if err != nil {
			t.Error(err)
		}

		w.Write(b)
	}))

	d := deployer.Deployer{Client: server.Client(), DeployServer: server.URL}

	exitCode, err := d.Run(cfg)
	assert.NoError(t, err)
	assert.Equal(t, exitCode, deployer.ExitSuccess)
}

func TestValidationFailures(t *testing.T) {
	for _, testCase := range []struct {
		errorMsg  string
		transform func(cfg deployer.Config) deployer.Config
	}{
		{deployer.RepositoryRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Repository = ""; return cfg }},
		{deployer.ClusterRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Cluster = ""; return cfg }},
		{deployer.APIKeyRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = ""; return cfg }},
		{deployer.ResourceRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Resource = nil; return cfg }},
		{deployer.MalformedAPIKeyMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = "malformed"; return cfg }},
		{deployer.MalformedAPIKeyMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = "malformed"; return cfg }},
	} {
		cfg := validConfig()
		cfg = testCase.transform(cfg)
		d := deployer.Deployer{}
		exitCode, err := d.Run(cfg)
		assert.Equal(t, exitCode, deployer.ExitInvocationFailure)
		assert.Contains(t, err.Error(), testCase.errorMsg)
	}
}

func validConfig() deployer.Config {
	cfg := deployer.NewConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}
	cfg.Cluster = "dev-fss"
	cfg.Repository = "org/asdf"
	cfg.APIKey = "1234567812345678"
	return cfg
}
