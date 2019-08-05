package circleci_sink

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/pkg/token-generator/types"
)

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Sink stores credentials as environment variables on CircleCI.
//
// https://circleci.com/docs/api/
func Sink(request types.TokenIssuerRequest, credentials types.Credentials, apiToken string, httpClient *http.Client) error {
	organization, repository, err := github.SplitFullname(request.Repository)
	if err != nil {
		return err
	}

	env := EnvVar{
		Name:  strings.ToUpper(fmt.Sprintf("%s_TOKEN", credentials.Source)),
		Value: credentials.Token,
	}

	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal environment variable payload: %s", err)
	}
	body := bytes.NewReader(payload)

	template := "https://circleci.com/api/v1.1/project/github/%s/%s/envvar"
	url := fmt.Sprintf(template, organization, repository)

	httpRequest, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	httpRequest = httpRequest.WithContext(request.Context)
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.SetBasicAuth(apiToken, "")

	httpResponse, err := httpClient.Do(httpRequest)
	if err != nil {
		return err
	}

	if httpResponse.StatusCode > 299 {
		respBody, _ := ioutil.ReadAll(httpResponse.Body)
		return fmt.Errorf("%s: %s", httpResponse.Status, string(respBody))
	}

	return nil
}
