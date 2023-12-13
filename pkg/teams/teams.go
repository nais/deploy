package teams

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"
)

type teamsClient struct {
	url        string
	httpClient *httpClient
}

func New(url, apiToken string) *teamsClient {
	return &teamsClient{
		url: url,
		httpClient: &httpClient{
			client:   http.DefaultClient,
			apiToken: apiToken,
		},
	}
}

func (t *teamsClient) IsAuthorized(repo, team string) bool {
	query := `query ($repoName: String! $teamSlug: Slug! $authorization: RepositoryAuthorization!) {
       isRepositoryAuthorized(repoName: $repoName, teamSlug: $teamSlug, authorization: $authorization)
	}`

	vars := map[string]string{
		"repoName":      repo,
		"teamSlug":      team,
		"authorization": "DEPLOY",
	}

	respBody := struct {
		Data struct {
			IsRepositoryAuthorized bool `json:"isRepositoryAuthorized"`
		} `json:"data"`
		Errors []map[string]any `json:"errors"`
	}{}

	if err := t.teamsQuery(context.Background(), query, vars, &respBody); err != nil {
		log.WithError(err).Error("checking repo authorization in teams")
		// TODO: error metrics
		return false
	}

	if len(respBody.Errors) > 0 {
		log.Errorf("checking repo authorization in teams: %v", respBody.Errors)
		// TODO: error metrics
		return false
	}

	return respBody.Data.IsRepositoryAuthorized
}

func (t *teamsClient) teamsQuery(ctx context.Context, query string, vars map[string]string, respBody interface{}) error {
	q := struct {
		Query     string            `json:"query"`
		Variables map[string]string `json:"variables"`
	}{
		Query:     query,
		Variables: vars,
	}

	body, err := json.Marshal(q)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.url, bytes.NewReader(body))
	if err != nil {
		return err
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, resp.Body)
		return fmt.Errorf("teams: %v", resp.Status)
	}

	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return err
	}

	return nil
}
