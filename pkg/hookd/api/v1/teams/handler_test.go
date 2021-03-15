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

	api_v1_teams "github.com/nais/deploy/pkg/hookd/api/v1/teams"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
)

type apiKeyStorage struct {
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

func (a *apiKeyStorage) RotateApiKey(ctx context.Context, team, groupId string, key []byte) error {
	return fmt.Errorf("err")
}

func (a *apiKeyStorage) ApiKeys(ctx context.Context, group string) (database.ApiKeys, error) {
	groups := make(database.ApiKeys, 0)

	switch group {
	case "group1-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team1",
			GroupId: "group1-claim",
			Expires: time.Time{},
			Created: time.Time{},
		})
	case "group2-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team2",
			GroupId: "group2-claim",
			Expires: time.Time{},
			Created: time.Time{},
		})
	case "group4-claim":
		groups = append(groups, database.ApiKey{
			Team:    "team4",
			GroupId: "group4-claim",
			Expires: time.Time{},
			Created: time.Time{},
		})
	}
	return groups, nil
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
