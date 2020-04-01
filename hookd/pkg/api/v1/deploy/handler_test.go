package api_v1_deploy_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/stretchr/testify/assert"
)

const (
	deploymentID = 123789
)

var secretKey = api_v1.Key{0xab, 0xcd, 0xef} // abcdef

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

type apiKeyStorage struct {
}

func (a *apiKeyStorage) ApiKeys(team string) (database.ApiKeys, error) {
	switch team {
	case "notfound":
		return nil, database.ErrNotFound
	case "unavailable":
		return nil, fmt.Errorf("service unavailable")
	default:
		return []database.ApiKey{{
			Key:     secretKey,
			Expires: time.Now().Add(1 * time.Hour),
		}}, nil
	}
}

func (a *apiKeyStorage) RotateApiKey(team, groupId string, key []byte) error {
	return nil
}

type deploymentStorage struct{}

func (s *deploymentStorage) Deployment(id string) (*database.Deployment, error) {
	return nil, nil
}

func (s *deploymentStorage) WriteDeployment(deployment database.Deployment) error {
	return nil
}

func (s *deploymentStorage) DeploymentStatus(deploymentID string) ([]database.DeploymentStatus, error) {
	return []database.DeploymentStatus{
		{
			ID:           "foo",
			DeploymentID: "123",
			Status:       "success",
			Message:      "all resources deployed",
		},
	}, nil
}

func (s *deploymentStorage) WriteDeploymentStatus(status database.DeploymentStatus) error {
	return nil
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
	request := httptest.NewRequest("POST", "/api/v1/deploy", bytes.NewReader(test.Request.Body))
	request.Header.Set("content-type", "application/json")

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(api_v1.SignatureHeader)) == 0 {
		mac := api_v1.GenMAC(test.Request.Body, secretKey)
		request.Header.Set(api_v1.SignatureHeader, hex.EncodeToString(mac))
	}

	requests := make(chan types.DeploymentRequest, 1024)
	statuses := make(chan types.DeploymentStatus, 1024)
	apiKeyStore := &apiKeyStorage{}
	deployStore := &deploymentStorage{}

	handler := api.New(api.Config{
		ApiKeyStore:     apiKeyStore,
		DeploymentStore: deployStore,
		Clusters:        validClusters,
		MetricsPath:     "/metrics",
		RequestChan:     requests,
		StatusChan:      statuses,
	})

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
