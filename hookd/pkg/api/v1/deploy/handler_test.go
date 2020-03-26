package api_v1_deploy_test

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

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"

	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/stretchr/testify/assert"
)

const (
	deploymentID = 123789
)

var secretKey = "foobar"

var validClusters = []string{
	"local",
}

type request struct {
	Headers map[string]string
	Body    json.RawMessage
}

type response struct {
	StatusCode int                              `json:"statusCode"`
	Body       api_v1_deploy.DeploymentResponse `json:"body"`
}

type testCase struct {
	Request  request  `json:"request"`
	Response response `json:"response"`
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

type apiKeyStorage struct {
	database.Database
}

func (a *apiKeyStorage) Read(team string) ([]database.ApiKey, error) {
	switch team {
	case "notfound":
		return nil, database.ErrNotFound
	case "unavailable":
		return nil, fmt.Errorf("service unavailable")
	default:
		return []database.ApiKey{{Key: secretKey}}, nil
	}
}

func (a *apiKeyStorage) Write(team, groupId string, key []byte) error {
	return nil
}

func (a *apiKeyStorage) Migrate() error {
	return nil
}

func (a *apiKeyStorage) IsErrNotFound(err error) bool {
	return err == database.ErrNotFound
}

func fileReader(file string) io.Reader {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	return f
}

func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	decodedBody := api_v1_deploy.DeploymentResponse{}
	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.NoError(t, err)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body.Message, decodedBody.Message)
	assert.NotEmpty(t, decodedBody.CorrelationID)

	assert.Equal(t, response.Body.GithubDeployment.GetID(), decodedBody.GithubDeployment.GetID())
}

func subTest(t *testing.T, name string) {
	inFile := fmt.Sprintf("testdata/%s", name)

	fixture := fileReader(inFile)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Error(data)
		t.Fail()
	}

	test := testCase{}
	err = json.Unmarshal(data, &test)
	if err != nil {
		t.Error(data)
		t.Fail()
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/", bytes.NewReader(test.Request.Body))

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(api_v1.SignatureHeader)) == 0 {
		mac := api_v1.GenMAC(test.Request.Body, []byte(secretKey))
		request.Header.Set(api_v1.SignatureHeader, hex.EncodeToString(mac))
	}

	requests := make(chan types.DeploymentRequest, 1024)
	statuses := make(chan types.DeploymentStatus, 1024)
	ghClient := githubClient{}
	apiKeyStore := apiKeyStorage{}

	handler := api_v1_deploy.DeploymentHandler{
		DeploymentRequest: requests,
		DeploymentStatus:  statuses,
		APIKeyStorage:     &apiKeyStore,
		GithubClient:      &ghClient,
		Clusters:          validClusters,
	}

	handler.ServeHTTP(recorder, request)

	testResponse(t, recorder, test.Response)
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
			subTest(t, name)
		})
	}
}
