package api_v1_teams_test

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

	api_v1_teams "github.com/navikt/deployment/hookd/pkg/api/v1/teams"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/stretchr/testify/assert"
)

type ApiKey struct {
	Team    string    `json:"Team"`
	GroupId string    `json:"groupId"`
	Key     string    `json:"key"`
	Expires time.Time `json:"expires"`
	Created time.Time `json:"created"`
}

type apiKeyStorage struct {
	database.Database
}
type testCase struct {
	Request  request  `json:"request"`
	Response response `json:"Response"`
}
type request struct {
	Headers map[string]string
	Body    json.RawMessage
	Groups  []string
}
type response struct {
	StatusCode int                 `json:"statusCode"`
	Body       []api_v1_teams.Team `json:"body"`
}

func (a *apiKeyStorage) Read(team string) ([]database.ApiKey, error) {
	return nil, fmt.Errorf("err")
}
func (a *apiKeyStorage) Write(team, groupId string, key []byte) error {
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
			Key:     "",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	case "group2-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team2",
			GroupId: "group2-claim",
			Key:     "",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	case "group4-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team4",
			GroupId: "group4-claim",
			Key:     "",
			Expires: time.Time{},
			Created: time.Time{},
		})
		return groups, nil
	default:
		return groups, nil
	}
}
func testResponse(t *testing.T, recorder *httptest.ResponseRecorder, response response) {

	body := []api_v1_teams.Team{}
	json.Unmarshal(recorder.Body.Bytes(), &body)
	assert.Equal(t, response.Body, body)
	assert.Equal(t, response.StatusCode, recorder.Code)

	//	assert.Equal(t, Response.Body.Team, body)
	if response.StatusCode == http.StatusOK {
		return
	}

	//	decodedBody := api_v1_provision.Response{}
	//	err := json.Unmarshal(recorder.Body.Bytes(), &decodedBody)
	//	assert.NoError(t, err)
	//	assert.Equal(t, Response.Body.Message, decodedBody.Message)
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

	test := testCase{}
	err = json.Unmarshal(data, &test)
	if err != nil {
		t.Error(string(data))
		t.Fail()
	}
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest("GET", "/api/v1/teams", bytes.NewReader(test.Request.Body))
	request = request.WithContext(context.WithValue(request.Context(), "groups", test.Request.Groups))

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}
	apiKeyStore := apiKeyStorage{}
	handler := api_v1_teams.TeamsHandler{
		APIKeyStorage: &apiKeyStore,
	}

	handler.ServeHTTP(recorder, request)
	testResponse(t, recorder, test.Response)
}
func TestProvisionHandler(t *testing.T) {
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
