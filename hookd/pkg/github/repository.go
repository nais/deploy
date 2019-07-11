package github

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/common/log"
	"github.com/shurcooL/githubv4"
)

var (
	repositories Repositories
)

type Repositories struct {
	sync.RWMutex
	List          []repo
	lastCreatedAt time.Time
}

type repo struct {
	Name                string    `json:"name"`
	NameWithOwner       string    `json:"full_name"`
	ViewerCanAdminister bool      `json:"-"`
	CreatedAt           time.Time `json:"-"`
}

func GetRepositories(client *githubv4.Client) (allRepos []repo, err error) {
	var query struct {
		Organization struct {
			Repositories struct {
				Nodes      []repo
				TotalCount githubv4.Int

				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"repositories(first:100, after: $repositoriesCursor)"`
		} `graphql:"organization(login: $organization)"`
	}

	variables := map[string]interface{}{
		"organization":       githubv4.String("navikt"),
		"repositoriesCursor": (*githubv4.String)(nil),
	}

	for {
		err = client.Query(context.Background(), &query, variables)
		if err != nil {
			log.Error(err)
			return
		}

		for _, repo := range query.Organization.Repositories.Nodes {
			allRepos = append(allRepos, repo)
		}

		if !query.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["repositoriesCursor"] = githubv4.NewString(query.Organization.Repositories.PageInfo.EndCursor)
	}
	return
}

func FilterRepositoriesByAdmin(allRepos []repo) (filteredRepos []repo) {
	for _, repo := range allRepos {
		if repo.ViewerCanAdminister {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return
}
