package api_v1_status_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/navikt/deployment/pkg/grpc/dispatchserver"
	"github.com/navikt/deployment/pkg/hookd/api"
	"github.com/navikt/deployment/pkg/hookd/api/v1"
	api_v1_status "github.com/navikt/deployment/pkg/hookd/api/v1/status"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type request struct {
	Headers map[string]string
	Body    json.RawMessage
}

type response struct {
	StatusCode int                          `json:"statusCode"`
	Body       api_v1_status.StatusResponse `json:"body"`
}

type testCase struct {
	Name     string
	Request  request
	Response response
	Setup    func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore)
}

func responseData(code int, status *string, message string) response {
	return response{
		StatusCode: code,
		Body: api_v1_status.StatusResponse{
			Status:  status,
			Message: message,
		},
	}
}

func stringp(s string) *string {
	return &s
}

var genericError = errors.New("oops")

var secretKey = api_v1.Key{0xab, 0xcd, 0xef} // abcdef

const deploymentID = "123789"

var validApiKeys = database.ApiKeys{
	database.ApiKey{
		Key:     secretKey,
		Expires: time.Now().Add(time.Hour * 1),
	},
}

var validPayload = []byte(`
{
	"team": "myteam",
	"deploymentID": "123789"
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
			Body: api_v1_status.StatusResponse{
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
			Body: api_v1_status.StatusResponse{
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
			Body: api_v1_status.StatusResponse{
				Message: "failed authentication",
			},
		},
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
		},
	},

	{
		Name: "Deployment ID missing from request",
		Request: request{
			Body: []byte(`
{
	"team": "myteam"
}
`),
		},
		Response: response{
			StatusCode: 400,
			Body: api_v1_status.StatusResponse{
				Message: "invalid status request: no deployment ID specified",
			},
		},
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, nil).Once()
		},
	},

	{
		Name: "API key not found",
		Request: request{
			Body: validPayload,
		},
		Response: response{
			StatusCode: 403,
			Body: api_v1_status.StatusResponse{
				Message: "failed authentication",
			},
		},
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, nil).Once()
		},
	},

	{
		Name: "API key service unavailable",
		Request: request{
			Body: validPayload,
		},
		Response: responseData(502, nil, "something wrong happened when communicating with api key service"),
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(database.ApiKeys{}, genericError).Once()
		},
	},

	{
		Name: "Database unavailable",
		Request: request{
			Body: validPayload,
		},
		Response: responseData(503, nil, "unable to determine deployment status; database is unavailable"),
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, deploymentID).Return(nil, genericError).Once()
		},
	},

	{
		Name: "Deployment not found",
		Request: request{
			Body: validPayload,
		},
		Response: responseData(404, nil, "deployment not found"),
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, deploymentID).Return(nil, database.ErrNotFound).Once()
		},
	},

	{
		Name: "Valid status request",
		Request: request{
			Body: validPayload,
		},
		Response: responseData(200, stringp("success"), "all resources deployed"),
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			apiKeyStore.On("ApiKeys", mock.Anything, "myteam").Return(validApiKeys, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, deploymentID).Return(
				[]database.DeploymentStatus{
					{
						ID:           "foo",
						DeploymentID: "123",
						Status:       "success",
						Message:      "all resources deployed",
					},
				},
				nil).Once()
		},
	},
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

func subTest(t *testing.T, test testCase) {
	test.Request.Body = addTimestampToBody(test.Request.Body, 0)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/v1/status", bytes.NewReader(test.Request.Body))
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
	deployServer := &dispatchserver.MockDispatchServer{}
	deployStore := &database.MockDeploymentStore{}

	if test.Setup != nil {
		test.Setup(deployServer, apiKeyStore, deployStore)
	}

	handler := api.New(api.Config{
		ApiKeyStore:     apiKeyStore,
		DispatchServer:  deployServer,
		DeploymentStore: deployStore,
		MetricsPath:     "/metrics",
	})

	handler.ServeHTTP(recorder, request)

	testResponse(t, recorder, test.Response)
}

func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	decodedBody := api_v1_status.StatusResponse{}
	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.NoError(t, err)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body.Message, decodedBody.Message)
	assert.Equal(t, response.Body.Status, decodedBody.Status)
}

// Deployment server integration tests using mocks; see table tests definitions above.
func TestStatusHandler_ServeHTTP(t *testing.T) {
	for _, test := range tests {
		t.Logf("Running test: %s", test.Name)
		subTest(t, test)
	}
}
