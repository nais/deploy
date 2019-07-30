package pusher

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// SetEnvironmentVariable sets an environment variable at CircleCI.
// https://circleci.com/docs/api/
func SetEnvironmentVariable(env EnvVar, organization, repository, apiToken string) error {
	payload, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal environment variable payload: %s", err)
	}
	body := bytes.NewReader(payload)

	template := "https://circleci.com/api/v1.1/project/github/%s/%s/envvar"
	url := fmt.Sprintf(template, organization, repository)

	request, err := http.NewRequest("POST", url, body)
	if err != nil {
		return err
	}

	request.Header.Set("Content-Type", "application/json")
	request.SetBasicAuth(apiToken, "")

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}

	if response.StatusCode > 299 {
		respBody, _ := ioutil.ReadAll(response.Body)
		log.Error(response.Status, string(respBody))
		return fmt.Errorf(response.Status)
	}

	return nil
}
