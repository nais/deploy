package api_v1_apikey_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/navikt/deployment/hookd/pkg/api"
	api_v1_apikey "github.com/navikt/deployment/hookd/pkg/api/v1/apikey"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/stretchr/testify/assert"
)

type ctxKey struct {
	name string
}

func (k ctxKey) String() string {
	return "context value " + k.name
}

type apiKeyStorage struct {
	database.Database
}

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

func tokenValidatorMiddleware(next http.Handler) http.Handler {
	return next
}

func (a *apiKeyStorage) Read(team string) ([]database.ApiKey, error) {
	teams := []database.ApiKey{}
	switch team {
	case "team1":
		teams = append(teams, database.ApiKey{
			Team:    "team1",
			GroupId: "group1-claim",
			Key:     "key1",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return teams, nil
	default:
		return nil, fmt.Errorf("err")
	}
}
func (a *apiKeyStorage) ReadAll(team, limit string) ([]database.ApiKey, error) {
	teams := []database.ApiKey{}
	switch team {
	case "team1":
		teams = append(teams, database.ApiKey{
			Team:    "team1",
			GroupId: "group1-claim",
			Key:     "key1",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return teams, nil
	default:
		return nil, fmt.Errorf("err")
	}
}

func (a *apiKeyStorage) Write(team, groupId string, key []byte) error {
	switch team {
	case "team1":
		return nil
	}
	return fmt.Errorf("err")
}

func (a *apiKeyStorage) IsErrNotFound(err error) bool {
	return err == persistence.ErrNotFound
}

func (a *apiKeyStorage) ReadByGroupClaim(group string) ([]database.ApiKey, error) {
	groups := []database.ApiKey{}
	switch group {
	case "group1-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team1",
			GroupId: "group1-claim",
			Key:     "key1",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	case "group2-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team2",
			GroupId: "group2-claim",
			Key:     "key2",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	case "group4-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team4",
			GroupId: "group4-claim",
			Key:     "key4",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	default:
		return groups, nil
	}
}
func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {
	body := []database.ApiKey{}
	json.Unmarshal(recorder.Body.Bytes(), &body)
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
	switch folder {
	case "GetApiKeys":
		request := httptest.NewRequest("GET", "/api/v1/teams", bytes.NewReader(test.Request.Body))
		request = request.WithContext(context.WithValue(request.Context(), "groups", test.Request.Groups))
		for key, val := range test.Request.Headers {
			request.Header.Set(key, val)
		}
		handler := api_v1_apikey.ApiKeyHandler{
			APIKeyStorage: &apiKeyStore,
		}
		handler.GetApiKeys(recorder, request)
		testResponse(t, recorder, test.Response)
	case "GetTeamApiKey":
		request := httptest.NewRequest("GET", "/api/v1/apikey/team1", bytes.NewReader(test.Request.Body))
		request = request.WithContext(context.WithValue(request.Context(), "groups", test.Request.Groups))
		for key, val := range test.Request.Headers {
			request.Header.Set(key, val)
		}
		handler := api.New(api.Config{
			MetricsPath:                 "/metrics",
			OAuthKeyValidatorMiddleware: tokenValidatorMiddleware,
			Database:                    &apiKeyStore,
		})
		handler.ServeHTTP(recorder, request)
		testResponse(t, recorder, test.Response)
	case "RotateTeamApiKey":
		request := httptest.NewRequest("POST", "/api/v1/apikey/team1", bytes.NewReader(test.Request.Body))
		request = request.WithContext(context.WithValue(request.Context(), "groups", test.Request.Groups))
		for key, val := range test.Request.Headers {
			request.Header.Set(key, val)
		}
		handler := api.New(api.Config{
			MetricsPath:                 "/metrics",
			OAuthKeyValidatorMiddleware: tokenValidatorMiddleware,
			Database:                    &apiKeyStore,
		})
		handler.ServeHTTP(recorder, request)
		testResponse(t, recorder, test.Response)
	}
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
