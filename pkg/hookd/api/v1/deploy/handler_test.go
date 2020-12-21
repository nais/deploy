package api_v1_deploy_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/navikt/deployment/pkg/grpc/deployserver"
	"github.com/navikt/deployment/pkg/hookd/api"
	"github.com/navikt/deployment/pkg/hookd/api/v1"
	"github.com/navikt/deployment/pkg/hookd/api/v1/deploy"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type request struct {
	Headers map[string]string
	Body    json.RawMessage
}

type response struct {
	StatusCode int
	Body       api_v1_deploy.DeploymentResponse
}

type testCase struct {
	Name     string
	Request  request
	Response response
	Setup    func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore)
}

func errorResponse(code int, message string) response {
	return response{
		StatusCode: code,
		Body: api_v1_deploy.DeploymentResponse{
			Message: message,
		},
	}
}

var genericError = errors.New("oops")

var secretKey = api_v1.Key{0xab, 0xcd, 0xef} // abcdef

var validApiKeys = database.ApiKeys{
	database.ApiKey{
		Key:     secretKey,
		Expires: time.Now().Add(time.Hour * 1),
	},
}

var validPayload = []byte(`
{
	"resources": [ {
		"kind": "ConfigMap",
		"version": "v1",
		"metadata": {
			"name": "foo",
			"namespace": "bar"
		}
	} ],
	"team": "myteam",
	"cluster": "local",
	"owner": "foo",
	"repository": "bar",
	"ref": "master",
	"environment": "baz"
}
`)

// Test case definitions
var tests = []testCase{
	{
		Name: "Empty request body",
		Request: request{
			Body: []byte(``),
		},
		Response: response{
			StatusCode: 400,
			Body: api_v1_deploy.DeploymentResponse{
				Message: "unable to unmarshal request body: unexpected end of JSON input",
			},
		},
	},

	{
		Name: "Invalid HMAC digest format",
		Request: request{
			Body: validPayload,
			Headers: map[string]string{
				"x-nais-signature": "foobar",
			},
		},
		Response: response{
			StatusCode: 400,
			Body: api_v1_deploy.DeploymentResponse{
				Message: "HMAC digest must be hex encoded",
			},
		},
	},

	{
		Name: "Wrong HMAC signature",
		Request: request{
			Body: validPayload,
			Headers: map[string]string{
				"x-nais-signature": "abcdef",
			},
		},
		Response: response{
			StatusCode: 403,
			Body: api_v1_deploy.DeploymentResponse{
				Message: "failed authentication",
			},
		},
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
		},
	},

	{
		Name: "API key not found",
		Request: request{
			Body: validPayload,
		},
		Response: response{
			StatusCode: 403,
			Body: api_v1_deploy.DeploymentResponse{
				Message: "failed authentication",
			},
		},
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, nil).Once()
		},
	},

	{
		Name: "API key service unavailable",
		Request: request{
			Body: validPayload,
		},
		Response: errorResponse(502, "something wrong happened when communicating with api key service"),
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, genericError).Once()
		},
	},

	{
		Name: "Database unavailable",
		Request: request{
			Body: validPayload,
		},
		Response: errorResponse(503, "database is unavailable; try again later"),
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("WriteDeployment", mock.Anything, mock.Anything).Return(genericError).Once()
		},
	},

	{
		Name: "Write deployment resource failed",
		Request: request{
			Body: validPayload,
		},
		Response: errorResponse(503, "database is unavailable; try again later"),
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("WriteDeployment", mock.Anything, mock.Anything).Return(genericError).Once()
			deployStore.On("WriteDeploymentResource", mock.Anything, mock.Anything, mock.Anything).Return(genericError).Once()
		},
	},

	{
		Name: "Deployd unavailable",
		Request: request{
			Body: validPayload,
		},
		Response: errorResponse(503, "deploy unavailable; try again later"),
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("WriteDeployment", mock.Anything, mock.Anything).Return(nil).Once()
			deployStore.On("WriteDeploymentResource", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			server.On("SendDeploymentRequest", mock.Anything, mock.Anything).Return(genericError).Once()
		},
	},

	{
		Name: "Valid deployment request",
		Request: request{
			Body: validPayload,
		},
		Response: errorResponse(201, "deployment request accepted and dispatched"),
		Setup: func(server *deployserver.MockDeployServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("WriteDeployment", mock.Anything, mock.Anything).Return(nil).Once()
			deployStore.On("WriteDeploymentResource", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			server.On("SendDeploymentRequest", mock.Anything, mock.Anything).Return(nil).Once()
			server.On("HandleDeploymentStatus", mock.Anything, mock.Anything).Return(nil).Once()
		},
	},
}

func subTest(t *testing.T, test testCase) {
	test.Request.Body = addTimestampToBody(test.Request.Body, 0)

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

	apiKeyStore := &database.MockApiKeyStore{}
	deployServer := &deployserver.MockDeployServer{}
	deployStore := &database.MockDeploymentStore{}

	if test.Setup != nil {
		test.Setup(deployServer, apiKeyStore, deployStore)
	}

	handler := api.New(api.Config{
		ApiKeyStore:     apiKeyStore,
		DeployServer:    deployServer,
		DeploymentStore: deployStore,
		MetricsPath:     "/metrics",
	})

	handler.ServeHTTP(recorder, request)

	testResponse(t, recorder, test.Response)
}

func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	decodedBody := api_v1_deploy.DeploymentResponse{}
	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.NoError(t, err)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body.Message, decodedBody.Message)
	assert.NotEmpty(t, decodedBody.CorrelationID)
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

// Deployment server integration tests using mocks; see table tests definitions above.
func TestDeploymentHandler_ServeHTTP(t *testing.T) {
	for _, test := range tests {
		t.Logf("Running test: %s", test.Name)
		subTest(t, test)
	}
}

// Test that certain fields missing from deployment request either errors out or validates.
func TestDeploymentRequest_Validate(t *testing.T) {
	req := &api_v1_deploy.DeploymentRequest{}

	errorTests := []func(){
		func() { req.Cluster = "" },
		func() { req.Ref = "" },
		func() { req.Team = "" },
		func() { req.Cluster = "" },
		func() { req.Resources = []byte(``) },
		func() { req.Resources = []byte(`"not a list"`) },
		func() { req.Resources = []byte(`{}`) },
	}
	successTests := []func(){
		func() { req.Owner = "" },
		func() { req.Repository = "" },
		func() { req.Owner = ""; req.Repository = "" },
	}

	setup := func() {
		err := json.Unmarshal(validPayload, req)
		if err != nil {
			panic(err)
		}
		req.Timestamp = api_v1.Timestamp(time.Now().Unix())
	}

	for _, setupFunc := range errorTests {
		setup()
		setupFunc()
		assert.Error(t, req.Validate())
	}

	for _, setupFunc := range successTests {
		setup()
		setupFunc()
		assert.NoError(t, req.Validate())
	}
}
