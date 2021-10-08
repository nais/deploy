package api_v1_dashboard_test

import (
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/nais/deploy/pkg/grpc/dispatchserver"
	"github.com/nais/deploy/pkg/hookd/api"
	api_v1_dashboard "github.com/nais/deploy/pkg/hookd/api/v1/dashboard"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type request struct {
}

type response struct {
	StatusCode int
	Body       api_v1_dashboard.DeploymentsResponse
}

type testCase struct {
	Name     string
	Request  request
	Response response
	Setup    func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore)
}

var genericError = errors.New("oops")

var timestamp = time.Now().UTC().Truncate(time.Microsecond)

// Test case definitions
var tests = []testCase{
	{
		Name: "Get all deployments",
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			deployStore.On("Deployments", mock.Anything, []string{}, 30).Return([]*database.Deployment{
				{ID: "1", Created: timestamp},
				{ID: "2", Created: timestamp},
			}, nil).Once()
			deployStore.On("Deployment", mock.Anything, "1").Return(&database.Deployment{ID: "1", Created: timestamp}, nil).Once()
			deployStore.On("Deployment", mock.Anything, "2").Return(&database.Deployment{ID: "2", Created: timestamp}, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, "1").Return([]database.DeploymentStatus{
				{ID: "1.1", Created: timestamp},
				{ID: "1.2", Created: timestamp},
			}, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, "2").Return([]database.DeploymentStatus{
				{ID: "2.1", Created: timestamp},
				{ID: "2.2", Created: timestamp},
			}, nil).Once()
			deployStore.On("DeploymentResources", mock.Anything, "1").Return([]database.DeploymentResource{
				{ID: "1.1"},
				{ID: "1.2"},
			}, nil).Once()
			deployStore.On("DeploymentResources", mock.Anything, "2").Return([]database.DeploymentResource{
				{ID: "2.1"},
				{ID: "2.2"},
			}, nil).Once()
		},
		Response: response{
			StatusCode: 200,
			Body: api_v1_dashboard.DeploymentsResponse{
				Deployments: []api_v1_dashboard.FullDeployment{
					{
						Deployment: database.Deployment{
							ID:      "1",
							Created: timestamp,
						},
						Statuses: []database.DeploymentStatus{
							{ID: "1.1", Created: timestamp},
							{ID: "1.2", Created: timestamp},
						},
						Resources: []database.DeploymentResource{
							{ID: "1.1"},
							{ID: "1.2"},
						},
					},
					{
						Deployment: database.Deployment{
							ID:      "2",
							Created: timestamp,
						},
						Statuses: []database.DeploymentStatus{
							{ID: "2.1", Created: timestamp},
							{ID: "2.2", Created: timestamp},
						},
						Resources: []database.DeploymentResource{
							{ID: "2.1"},
							{ID: "2.2"},
						},
					},
				},
			},
		},
	},

	{
		Name: "Database failing on first query",
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			deployStore.On("Deployments", mock.Anything, []string{}, 30).Return(nil, genericError)
		},
		Response: response{
			StatusCode: 500,
		},
	},

	{
		Name: "Database failing on deployment query",
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			deployStore.On("Deployments", mock.Anything, []string{}, 30).Return([]*database.Deployment{{ID: "1", Created: timestamp}}, nil).Once()
			deployStore.On("Deployment", mock.Anything, "1").Return(nil, genericError)
		},
		Response: response{
			StatusCode: 500,
		},
	},

	{
		Name: "Database failing on status query",
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			deployStore.On("Deployments", mock.Anything, []string{}, 30).Return([]*database.Deployment{{ID: "1", Created: timestamp}}, nil).Once()
			deployStore.On("Deployment", mock.Anything, "1").Return(&database.Deployment{ID: "1", Created: timestamp}, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, "1").Return(nil, genericError)
		},
		Response: response{
			StatusCode: 500,
		},
	},

	{
		Name: "Database failing on deployment query",
		Setup: func(server *dispatchserver.MockDispatchServer, apiKeyStore *database.MockApiKeyStore, deployStore *database.MockDeploymentStore) {
			deployStore.On("Deployments", mock.Anything, []string{}, 30).Return([]*database.Deployment{{ID: "1", Created: timestamp}}, nil).Once()
			deployStore.On("Deployment", mock.Anything, "1").Return(&database.Deployment{ID: "1", Created: timestamp}, nil).Once()
			deployStore.On("DeploymentStatus", mock.Anything, "1").Return([]database.DeploymentStatus{{ID: "1.1", Created: timestamp}}, nil).Once()
			deployStore.On("DeploymentResources", mock.Anything, "1").Return(nil, genericError).Once()
		},
		Response: response{
			StatusCode: 500,
		},
	},
}

func subTest(t *testing.T, test testCase) {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/dashboard/deployments", nil)
	request.Header.Set("content-type", "application/json")

	apiKeyStore := &database.MockApiKeyStore{}
	deployServer := &dispatchserver.MockDispatchServer{}
	deployStore := &database.MockDeploymentStore{}

	if test.Setup != nil {
		test.Setup(deployServer, apiKeyStore, deployStore)
	}

	handler := api.New(api.Config{
		ApiKeyStore:                 apiKeyStore,
		DispatchServer:              deployServer,
		DeploymentStore:             deployStore,
		OAuthKeyValidatorMiddleware: middleware.WithValue("foo", nil),
		MetricsPath:                 "/metrics",
	})

	handler.ServeHTTP(recorder, request)

	testResponse(t, recorder, test.Response)
}

func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	decodedBody := api_v1_dashboard.DeploymentsResponse{}
	_ = json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body.Deployments, decodedBody.Deployments)
}

// Deployment server integration tests using mocks; see table tests definitions above.
func TestDashboardHandler_Deployments(t *testing.T) {
	for _, test := range tests {
		t.Logf("Running test: %s", test.Name)
		subTest(t, test)
	}
}
