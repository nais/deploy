package circleci_sink_test

import (
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/navikt/deployment/pkg/token-generator/sinks/circleci"
	"github.com/navikt/deployment/pkg/token-generator/types"
	"github.com/stretchr/testify/assert"
)

const (
	apiToken    = "my api token"
	githubToken = "v1.something"
	repository  = "org/myrepository"
	url         = "https://circleci.com/api/v1.1/project/github/org/myrepository/envvar"
)

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

// NewTestClient returns *http.Client with Transport replaced to avoid making real calls
func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

// Check that the CircleCI client uses the correct URL, API key
func TestSink(t *testing.T) {
	httpClient := NewTestClient(func(req *http.Request) *http.Response {

		user, pass, ok := req.BasicAuth()
		body, err := ioutil.ReadAll(req.Body)

		assert.NoError(t, err)
		assert.Truef(t, ok, "basic auth used to identify user")
		assert.Equal(t, apiToken, user)
		assert.Equal(t, "", pass)
		assert.Equal(t, url, req.URL.String())
		assert.Equal(t, `{"name":"GITHUB_TOKEN","value":"v1.something"}`, string(body))

		return &http.Response{
			StatusCode: http.StatusOK,
		}
	})

	request := types.Request{
		Repository: repository,
	}
	credentials := types.Credentials{
		Source: "github",
		Token:  githubToken,
	}

	err := circleci_sink.Sink(request, credentials, apiToken, httpClient)
	assert.NoError(t, err)
}
