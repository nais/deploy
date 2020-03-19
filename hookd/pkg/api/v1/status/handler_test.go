package api_v1_status_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/status"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/stretchr/testify/assert"
)

var secretKey = []byte("foobar")

const (
	deploymentID = 123789
)

type statusRequest struct {
	Headers map[string]string
	Body    json.RawMessage
}

type statusResponse struct {
	StatusCode int                          `json:"statusCode"`
	Body       api_v1_status.StatusResponse `json:"body"`
}

type statusTestCase struct {
	Request  statusRequest  `json:"request"`
	Response statusResponse `json:"response"`
}

type githubClient struct{}

func (g *githubClient) TeamAllowed(ctx context.Context, owner, repository, teamName string) error {
	switch teamName {
	case "team_not_repo_owner":
		return github.ErrTeamNoAccess
	case "team_not_on_github":
		return github.ErrTeamNotExist
	default:
		return nil
	}
}

func (g *githubClient) CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error) {
	switch repository {
	case "unavailable":
		return nil, fmt.Errorf("GitHub is down!")
	default:
		return &gh.Deployment{
			ID: gh.Int64(deploymentID),
		}, nil
	}
}

func (g *githubClient) DeploymentStatus(ctx context.Context, owner, repository string, deploymentID int64) (*gh.DeploymentStatus, error) {
	return &gh.DeploymentStatus{
		ID:    gh.Int64(deploymentID),
		State: gh.String("success"),
	}, nil
}

type apiKeyStorage struct{}

func (a *apiKeyStorage) Read(team string) ([][]byte, error) {
	switch team {
	case "notfound":
		return nil, persistence.ErrNotFound
	case "unavailable":
		return nil, fmt.Errorf("service unavailable")
	default:
		return [][]byte{secretKey}, nil
	}
}

func (a *apiKeyStorage) Write(team string, key []byte) error {
	return nil
}

func (a *apiKeyStorage) Migrate() error {
	return nil
}

func (a *apiKeyStorage) IsErrNotFound(err error) bool {
	return err == persistence.ErrNotFound
}

func testStatusResponse(t *testing.T, recorder *httptest.ResponseRecorder, response statusResponse) {
	decodedBody := api_v1_status.StatusResponse{}
	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.NoError(t, err)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body.Message, decodedBody.Message)
	assert.Equal(t, response.Body.Status, decodedBody.Status)
}

// Inject timestamp in request payload
func addTimestampToBody(in []byte, timeshift int64) []byte {
	tmp := make(map[string]interface{})
	err := json.Unmarshal(in, &tmp)
	if err != nil {
		return in
	}
	if _, ok := tmp["timestamp"]; ok {
		// timestamp already provided in test fixture
		return in
	}
	tmp["timestamp"] = time.Now().Unix() + timeshift
	out, err := json.Marshal(tmp)
	if err != nil {
		return in
	}
	return out
}

func fileReader(file string) io.Reader {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	return f
}

func statusSubTest(t *testing.T, name string) {
	inFile := fmt.Sprintf("testdata/%s", name)

	fixture := fileReader(inFile)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Error(data)
		t.Fail()
	}

	test := statusTestCase{}
	err = json.Unmarshal(data, &test)
	if err != nil {
		t.Error(string(data))
		t.Fail()
	}

	body := addTimestampToBody(test.Request.Body, 0)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/status", bytes.NewReader(body))

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(api_v1.SignatureHeader)) == 0 {
		mac := api_v1.GenMAC(body, secretKey)
		request.Header.Set(api_v1.SignatureHeader, hex.EncodeToString(mac))
	}

	ghClient := githubClient{}
	apiKeyStore := apiKeyStorage{}

	handler := api_v1_status.StatusHandler{
		APIKeyStorage: &apiKeyStore,
		GithubClient:  &ghClient,
	}

	handler.ServeHTTP(recorder, request)

	testStatusResponse(t, recorder, test.Response)
}

func TestHandler(t *testing.T) {
	files, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		t.Run(name, func(t *testing.T) {
			statusSubTest(t, name)
		})
	}
}
