package server_test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/navikt/deployment/hookd/pkg/server"
	"github.com/stretchr/testify/assert"
)

type statusRequest struct {
	Headers map[string]string
	Body    json.RawMessage
}

type statusResponse struct {
	StatusCode int                   `json:"statusCode"`
	Body       server.StatusResponse `json:"body"`
}

type statusTestCase struct {
	Request  statusRequest  `json:"request"`
	Response statusResponse `json:"response"`
}

func testStatusResponse(t *testing.T, recorder *httptest.ResponseRecorder, response statusResponse) {
	decodedBody := server.StatusResponse{}
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

func statusSubTest(t *testing.T, name string) {
	inFile := fmt.Sprintf("testdata/status/%s", name)

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
	request := httptest.NewRequest("GET", "/api/v1/status", bytes.NewReader(body))

	for key, val := range test.Request.Headers {
		request.Header.Set(key, val)
	}

	// Generate HMAC header for cases where the header should be valid
	if len(request.Header.Get(server.SignatureHeader)) == 0 {
		mac := server.GenMAC(body, secretKey)
		request.Header.Set(server.SignatureHeader, hex.EncodeToString(mac))
	}

	ghClient := githubClient{}
	apiKeyStore := apiKeyStorage{}

	handler := server.StatusHandler{
		APIKeyStorage: &apiKeyStore,
		GithubClient:  &ghClient,
	}

	handler.ServeHTTP(recorder, request)

	testStatusResponse(t, recorder, test.Response)
}

func TestStatusHandler(t *testing.T) {
	files, err := ioutil.ReadDir("testdata/status")
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
