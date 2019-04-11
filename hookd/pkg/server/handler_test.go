package server_test

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v23/github"
	"github.com/google/uuid"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/server"
	"github.com/stretchr/testify/assert"
)

const (
	queueSize = 32
)

var (
	secretToken      = "abc"
	wrongSecretToken = "wrong"
)

type mockRepository struct {
	Contents map[string][]string
}

func (s *mockRepository) Read(repository string) ([]string, error) {
	return []string{}, nil
}

func (s *mockRepository) Write(repository string, teams []string) error {
	return nil
}

func (s *mockRepository) IsErrNotFound(err error) bool {
	return false
}

type handlerTest struct {
	Handler  *server.DeploymentHandler
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

func newHandler() *server.DeploymentHandler {
	requestChan := make(chan deployment.DeploymentRequest, queueSize)
	statusChan := make(chan deployment.DeploymentStatus, queueSize)

	store := &mockRepository{
		Contents: make(map[string][]string, 0),
	}

	return &server.DeploymentHandler{
		DeploymentRequest:     requestChan,
		DeploymentStatus:      statusChan,
		TeamRepositoryStorage: store,
		SecretToken:           secretToken,
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
		dr := newDeploymentEvent("foo/bar", "env", "{}")
		b, _ := json.Marshal(dr)
		ht.Body.Write(b)
		ht.Sign(secretToken)
		ht.Run()

		assert.Equal(t, http.StatusBadRequest, ht.Recorder.Code)
		assert.Equal(t, "payload signature check failed", ht.Recorder.Body.String())
	})
}
