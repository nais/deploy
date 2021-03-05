package deployer_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/navikt/deployment/pkg/deployer"
	"github.com/navikt/deployment/pkg/pb"
)
/*
	Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, nil).Once()
		},
*/
func TestHappyPath(t *testing.T) {
	cfg := validConfig()

	client := &pb.MockDeployClient{}
	client.On("Deploy", mock.Anything, mock.Anything).Return(&pb.DeploymentStatus{
		Request: &pb.DeploymentRequest{
			ID:                "1",
		},
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "happy happy happy",
	}, nil)
	d := deployer.Deployer{Client: client}

	exitCode, err := d.Run(cfg)
	assert.NoError(t, err)
	assert.Equal(t, exitCode, deployer.ExitSuccess)
}
/*
func TestHappyPathForAlert(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/alert.json"}
	expectedOutput, err := ioutil.ReadFile(cfg.Resource[0])
	if err != nil {
		assert.NoError(t, err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)

		deployRequest := api_v1_deploy.DeploymentRequest{}

		if err := json.NewDecoder(r.Body).Decode(&deployRequest); err != nil {
			t.Error(err)
		}

		assert.Equal(t, deployRequest.Team, "aura", "auto-detection of team works")
		assert.Equal(t, deployRequest.Ref, "master", "defaulting works")
		assert.Equal(t, deployRequest.Environment, "dev-fss", "auto-detection of environment works")

		resources := make([]json.RawMessage, len(cfg.Resource))
		resources[0] = expectedOutput
		expectedBytes, err := json.MarshalIndent(resources, "  ", "  ")

		assert.NoError(t, err)
		assert.Equal(t, string(expectedBytes), string(deployRequest.Resources))

		b, err := json.Marshal(&api_v1_deploy.DeploymentResponse{})

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
*/
/*
func TestWaitForComplete(t *testing.T) {
	requests := 0
	cfg := validConfig()
	cfg.Wait = true
	cfg.PollInterval = time.Millisecond * 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		marshaler := json.NewEncoder(w)
		switch r.RequestURI {
		case "/api/v1/deploy":
			w.WriteHeader(http.StatusCreated)
			marshaler.Encode(&api_v1_deploy.DeploymentResponse{})
		case "/api/v1/status":
			var status string
			w.WriteHeader(http.StatusOK)
			switch requests {
			case 0:
				status = pb.DeploymentState_pending.String()
			case 1:
				status = pb.DeploymentState_in_progress.String()
			case 2:
				status = pb.DeploymentState_success.String()
			}
			requests++
			marshaler.Encode(&api_v1_status.StatusResponse{
				Status: &status,
			})
		}
	}))

	d := deployer.Deployer{Client: server.Client(), DeployServer: server.URL}

	exitCode, err := d.Run(cfg)
	assert.NoError(t, err)
	assert.Equal(t, exitCode, deployer.ExitSuccess)
}

func TestWaitForTheInevitableEventualFailure(t *testing.T) {
	requests := 0
	cfg := validConfig()
	cfg.Wait = true
	cfg.PollInterval = time.Millisecond * 1

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		marshaler := json.NewEncoder(w)
		switch r.RequestURI {
		case "/api/v1/deploy":
			w.WriteHeader(http.StatusCreated)
			marshaler.Encode(&api_v1_deploy.DeploymentResponse{})
		case "/api/v1/status":
			var status string
			w.WriteHeader(http.StatusOK)
			switch requests {
			case 0:
				status = pb.DeploymentState_pending.String()
			case 1:
				status = pb.DeploymentState_in_progress.String()
			case 2:
				status = pb.DeploymentState_failure.String()
			}
			requests++
			marshaler.Encode(&api_v1_status.StatusResponse{
				Status: &status,
			})
		}
	}))

	d := deployer.Deployer{Client: server.Client(), DeployServer: server.URL}

	exitCode, err := d.Run(cfg)
	assert.NoError(t, err)
	assert.Equal(t, exitCode, deployer.ExitDeploymentFailure)
}

func TestValidationFailures(t *testing.T) {
	for _, testCase := range []struct {
		errorMsg  string
		transform func(cfg deployer.Config) deployer.Config
	}{
		{deployer.ClusterRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Cluster = ""; return cfg }},
		{deployer.APIKeyRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = ""; return cfg }},
		{deployer.ResourceRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Resource = nil; return cfg }},
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
*/
func TestMultiDocumentParsing(t *testing.T) {
	docs, err := deployer.MultiDocumentFileAsJSON("testdata/multi_document.yaml", deployer.TemplateVariables{})
	assert.Len(t, docs, 2)
	assert.NoError(t, err)
	assert.Equal(t, `{"document":1}`, string(docs[0]))
	assert.Equal(t, `{"document":2}`, string(docs[1]))
}

func TestMultiDocumentTemplating(t *testing.T) {
	ctx := deployer.TemplateVariables{
		"ingresses": []string{
			"https://foo",
			"https://bar",
		},
	}
	docs, err := deployer.MultiDocumentFileAsJSON("testdata/templating.yaml", ctx)
	assert.Len(t, docs, 2)
	assert.NoError(t, err)
	assert.Equal(t, `{"ingresses":["https://foo","https://bar"]}`, string(docs[0]))
	assert.Equal(t, `{"ungresses":["https://foo","https://bar"]}`, string(docs[1]))
}

func TestExitCodeZero(t *testing.T) {
	assert.Equal(t, deployer.ExitCode(0), deployer.ExitSuccess)
}

func validConfig() deployer.Config {
	cfg := deployer.NewConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}
	cfg.Cluster = "dev-fss"
	cfg.Repository = "myrepo"
	cfg.APIKey = "1234567812345678"
	return cfg
}