package server_test

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v27/github"
	"github.com/google/uuid"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/server"
	"github.com/stretchr/testify/assert"
)

var (
	secretToken      = "abc"
	wrongSecretToken = "wrong"
	validClusters    = []string{
		"local",
	}
)

type mockRepository struct {
	Contents map[string][]string
}

func (s *mockRepository) ReadRepositoryTeams(ctx context.Context, repository string) ([]string, error) {
	return []string{}, nil
}

func (s *mockRepository) WriteRepositoryTeams(ctx context.Context, repository string, teams []string) error {
	return nil
}

type borker struct{}

func (b *borker) SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error {
	return nil
}

func (b *borker) HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error {
	return nil
}

func (b *borker) Deployments(deploymentOpts *deployment.GetDeploymentOpts, deploymentsServer deployment.Deploy_DeploymentsServer) error {
	return nil
}

func (b *borker) ReportStatus(ctx context.Context, status *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, nil
}

func (b *borker) Queue(request *deployment.DeploymentRequest) error {
	return nil
}

type handlerTest struct {
	Handler  *server.GithubDeploymentHandler
	Body     *bytes.Buffer
	Request  *http.Request
	Recorder *httptest.ResponseRecorder
}

func (h *handlerTest) Run() {
	h.Handler.ServeHTTP(h.Recorder, h.Request)
}

func (h *handlerTest) Sign(key string) {
	hasher := hmac.New(sha1.New, []byte(key))
	hasher.Write(h.Body.Bytes())
	sum := hasher.Sum(nil)
	h.Request.Header.Set("X-Hub-Signature", fmt.Sprintf("sha1=%s", hex.EncodeToString(sum)))
}

func newHandler() *server.GithubDeploymentHandler {
	store := &mockRepository{
		Contents: make(map[string][]string, 0),
	}

	return &server.GithubDeploymentHandler{
		Broker:                &borker{},
		TeamRepositoryStorage: store,
		SecretToken:           secretToken,
		Clusters:              validClusters,
	}
}

func newDeploymentEvent(repoName, environment, payload string) *gh.DeploymentEvent {
	return &gh.DeploymentEvent{
		Repo: &gh.Repository{
			FullName: gh.String(repoName),
		},
		Deployment: &gh.Deployment{
			Environment: gh.String(environment),
			Payload:     []byte(payload),
		},
	}
}

func setup() handlerTest {
	buf := make([]byte, 0)
	ht := handlerTest{
		Handler:  newHandler(),
		Body:     bytes.NewBuffer(buf),
		Recorder: httptest.NewRecorder(),
	}
	ht.Request = httptest.NewRequest("POST", "/events", ht.Body)
	ht.Request.Header.Set("X-GitHub-Delivery", uuid.New().String())
	ht.Request.Header.Set("X-GitHub-Event", "deployment")
	ht.Request.Header.Set("content-type", "application/json")
	return ht
}

func TestDeploymentHandler_ServeHTTP(t *testing.T) {

	t.Run("unsupported event types are silently ignored", func(t *testing.T) {
		ht := setup()
		ht.Request.Header.Set("X-GitHub-Event", "ping")
		ht.Sign(secretToken)
		ht.Run()

		assert.Equal(t, http.StatusNoContent, ht.Recorder.Code)
	})

	t.Run("deployment events without signature are rejected", func(t *testing.T) {
		ht := setup()
		ht.Body.WriteString("{}")
		ht.Run()

		assert.Equal(t, http.StatusForbidden, ht.Recorder.Code)
		assert.Equal(t, "missing signature", ht.Recorder.Body.String())
	})

	t.Run("deployment events with wrong signature are rejected", func(t *testing.T) {
		ht := setup()
		ht.Body.WriteString("{}")
		ht.Sign(wrongSecretToken)
		ht.Run()

		assert.Equal(t, http.StatusForbidden, ht.Recorder.Code)
		assert.Equal(t, "payload signature check failed", ht.Recorder.Body.String())
	})

	t.Run("malformed deployment events are rejected", func(t *testing.T) {
		ht := setup()
		ht.Body.WriteString("foo and bar")
		ht.Sign(secretToken)
		ht.Run()

		assert.Equal(t, http.StatusBadRequest, ht.Recorder.Code)
	})

	t.Run("deployment requests without team in payload are rejected", func(t *testing.T) {
		ht := setup()
		dr := newDeploymentEvent("foo/bar", "local", "{}")
		b, _ := json.Marshal(dr)
		ht.Body.Write(b)
		ht.Sign(secretToken)
		ht.Run()

		assert.Equal(t, http.StatusBadRequest, ht.Recorder.Code)
		assert.Equal(t, "no team was specified in deployment payload", ht.Recorder.Body.String())
	})
}
