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

	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/status"
	"github.com/navikt/deployment/hookd/pkg/database"
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

type apiKeyStorage struct {
}

func (a *apiKeyStorage) ApiKeys(ctx context.Context, team string) (database.ApiKeys, error) {
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

func (a *apiKeyStorage) RotateApiKey(ctx context.Context, team, groupId string, key []byte) error {
	return nil
}

type deploymentStorage struct{}

func (s *deploymentStorage) Deployment(ctx context.Context, id string) (*database.Deployment, error) {
	return nil, nil
}

func (s *deploymentStorage) WriteDeployment(ctx context.Context, deployment database.Deployment) error {
	return nil
}

func (s *deploymentStorage) DeploymentStatus(ctx context.Context, deploymentID string) ([]database.DeploymentStatus, error) {
	return []database.DeploymentStatus{
		{
			ID:           "foo",
			DeploymentID: "123",
			Status:       "success",
			Message:      "all resources deployed",
		},
	}, nil
}

func (s *deploymentStorage) WriteDeploymentStatus(ctx context.Context, status database.DeploymentStatus) error {
	return nil
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
	request := httptest.NewRequest("POST", "/api/v1/status", bytes.NewReader(body))
	request.Header.Set("content-type", "application/json")

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(api_v1.SignatureHeader)) == 0 {
		mac := api_v1.GenMAC(body, secretKey)
		request.Header.Set(api_v1.SignatureHeader, hex.EncodeToString(mac))
	}

	apiKeyStore := &apiKeyStorage{}
	statusStore := &deploymentStorage{}

	handler := api.New(api.Config{
		ApiKeyStore:     apiKeyStore,
		DeploymentStore: statusStore,
		MetricsPath:     "/metrics",
	})

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
