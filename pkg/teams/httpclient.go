package teams

import (
	"net/http"
)

type httpClient struct {
	client   *http.Client
	apiToken string
}

func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return c.client.Do(req)
}
