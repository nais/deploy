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
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/api"
	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/stretchr/testify/assert"
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

type borker struct{}

func (b *borker) SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error {
	switch deployment.GetPayloadSpec().GetTeam() {
	case "kafka_unavailable":
		return fmt.Errorf("deploy queue is unavailable; try again later")
	}
	return nil
}

func (b *borker) HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error {
	return nil
}

type db struct{}

func (db *db) ApiKeys(ctx context.Context, team string) (database.ApiKeys, error) {
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

func (db *db) RotateApiKey(ctx context.Context, team, groupId string, key []byte) error {
	return nil
}

func (db *db) Deployment(ctx context.Context, id string) (*database.Deployment, error) {
	return nil, nil
}

func (db *db) WriteDeployment(ctx context.Context, deployment database.Deployment) error {
	switch deployment.Team {
	case "database_unavailable":
		return fmt.Errorf("oops")
	}
	return nil
}

func (db *db) DeploymentStatus(ctx context.Context, deploymentID string) ([]database.DeploymentStatus, error) {
	return []database.DeploymentStatus{
		{
			ID:           "foo",
			DeploymentID: "123",
			Status:       "success",
			Message:      "all resources deployed",
		},
	}, nil
}

func (db *db) WriteDeploymentStatus(ctx context.Context, status database.DeploymentStatus) error {
	return nil
}

func (b *borker) Deployments(deploymentOpts *deployment.GetDeploymentOpts, deploymentsServer deployment.Deploy_DeploymentsServer) error {
	return nil
}

func (b *borker) ReportStatus(ctx context.Context, status *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, nil
}

func (b *borker) Queue(request *deployment.DeploymentRequest) {}

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

	apiKeyStore := &db{}
	brok := &borker{}

	handler := api.New(api.Config{
		ApiKeyStore:     apiKeyStore,
		DeployServer:    brok,
		DeploymentStore: apiKeyStore,
		Clusters:        validClusters,
		MetricsPath:     "/metrics",
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
