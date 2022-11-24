package api_v1_apikey_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/nais/deploy/pkg/hookd/middleware"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi"

	"github.com/nais/deploy/pkg/hookd/api"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
)

type apiKeyStorage struct{}

type testCase struct {
	Request  request  `json:"request"`
	Response response `json:"Response"`
}

type request struct {
	Headers      map[string]string
	Body         json.RawMessage
	Groups       []string
	Team         string
	RouteContext map[string]string
}

type response struct {
	StatusCode int               `json:"statusCode"`
	Body       []database.ApiKey `json:"body"`
}

var (
	key1 = api_v1.Key{0xab, 0xcd, 0xef} // abcdef
	key2 = api_v1.Key{0x12, 0x34, 0x56} // 123456
	key4 = api_v1.Key{0x00}             // not used
)

func tokenValidatorMiddleware(next http.Handler) http.Handler {
	return next
}

func (a *apiKeyStorage) ApiKeys(ctx context.Context, id string) (database.ApiKeys, error) {
	switch id {
	case "team1":
		return database.ApiKeys{{
			Team:    "team1",
			GroupId: "group1-claim",
			Key:     key1,
		}}, nil
	case "group1-claim":
		return database.ApiKeys{{
			Team:    "team1",
			GroupId: "group1-claim",
			Key:     key1,
		}}, nil
	case "group2-claim":
		return database.ApiKeys{{
			Team:    "team2",
			GroupId: "group2-claim",
			Key:     key2,
		}}, nil
	case "group4-claim":
		return database.ApiKeys{{
			Team:    "team4",
			GroupId: "group4-claim",
			Key:     key4,
		}}, nil
	default:
		return nil, fmt.Errorf("err")
	}
}

func (a *apiKeyStorage) RotateApiKey(ctx context.Context, team, groupId string, key []byte) error {
	switch team {
	case "team1":
		return nil
	}
	return fmt.Errorf("err")
}

func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	body := make([]database.ApiKey, 0)
	decoder := json.NewDecoder(recorder.Body)
	_ = decoder.Decode(&body)
	assert.Equal(t, response.StatusCode, recorder.Code)
	assert.Equal(t, response.Body, body)
	return
}

func fileReader(file string) io.Reader {
	f, err := os.Open(file)
	if err != nil {
		panic(err)
	}
	return f
}

func statusSubTest(t *testing.T, folder, file string) {
	var request *http.Request
	inFile := fmt.Sprintf("testdata/%s/%s", folder, file)

	fixture := fileReader(inFile)
	data, err := ioutil.ReadAll(fixture)
	if err != nil {
		t.Error(data)
		t.Fail()
	}

	test := testCase{}
	err = json.Unmarshal(data, &test)
	if err != nil {
		t.Error(string(data))
		t.Fail()
	}
	recorder := httptest.NewRecorder()
	apiKeyStore := apiKeyStorage{}
	handler := api.New(api.Config{
		GroupProvider:        middleware.GroupProviderAzure,
		ApiKeyStore:          &apiKeyStore,
		MetricsPath:          "/metrics",
		ValidatorMiddlewares: chi.Middlewares{tokenValidatorMiddleware},
	})

	switch folder {
	case "GetApiKeys":
		request = httptest.NewRequest("GET", "/api/v1/teams", bytes.NewReader(test.Request.Body))
	case "GetTeamApiKey":
		request = httptest.NewRequest("GET", "/api/v1/apikey/team1", bytes.NewReader(test.Request.Body))
	case "RotateTeamApiKey":
		request = httptest.NewRequest("POST", "/api/v1/apikey/team1", bytes.NewReader(test.Request.Body))
	default:
		panic("unhandled test case")
	}

	request = request.WithContext(context.WithValue(request.Context(), "groups", test.Request.Groups))
	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}
	handler.ServeHTTP(recorder, request)
	testResponse(t, recorder, test.Response)
}

func TestApiKeyHandler(t *testing.T) {
	subFolders, err := ioutil.ReadDir("testdata")
	if err != nil {
		t.Error(err)
		t.Fail()
	}
	for _, folder := range subFolders {
		if folder.IsDir() {
			testfiles, err := ioutil.ReadDir(fmt.Sprintf("testdata/%s", folder.Name()))
			if err != nil {
				t.Error(err)
				t.Fail()
			}
			for _, file := range testfiles {
				t.Run(fmt.Sprintf("%s/%s", folder.Name(), file.Name()), func(t *testing.T) {
					statusSubTest(t, folder.Name(), file.Name())
				})
			}
		}
	}
}
