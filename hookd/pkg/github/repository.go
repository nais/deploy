package github

import (
	"context"
	"fmt"
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

func GetRepositories(client *githubv4.Client) (repos []repo, err error) {
	var query struct {
		Organization struct {
			Repositories struct {
				Nodes      []repo
				TotalCount int

				PageInfo struct {
					EndCursor   githubv4.String
					HasNextPage bool
				}
			} `graphql:"repositories(first:100, after: $repositoriesCursor, orderBy: {field: $field, direction:$dir})"`
		} `graphql:"organization(login: $organization)"`
	}

	variables := map[string]interface{}{
		"organization":       githubv4.String("navikt"),
		"repositoriesCursor": (*githubv4.String)(nil),
		"field":              githubv4.RepositoryOrderFieldCreatedAt,
		"dir":                githubv4.OrderDirectionDesc,
	}

Loop:
	for {
		err = client.Query(context.Background(), &query, variables)
		if err != nil {
			log.Error(err)
			return repos, err
		}

		repositories.RLock()
		for _, repo := range query.Organization.Repositories.Nodes {
			if repo.CreatedAt.After(repositories.lastCreatedAt) {
				repos = append(repos, repo)
			} else {
				repositories.RUnlock()
				break Loop
			}
		}
		repositories.RUnlock()

		if !query.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["repositoriesCursor"] = githubv4.NewString(query.Organization.Repositories.PageInfo.EndCursor)
	}

	repositories.Lock()
	defer repositories.Unlock()

	allRepos := append(repos, repositories.List...)
	if len(allRepos) != 0 && len(allRepos) == query.Organization.Repositories.TotalCount {
		repositories.lastCreatedAt = allRepos[0].CreatedAt
		repositories.List = allRepos
	} else {
		repositories.lastCreatedAt = time.Time{}
		repositories.List = []repo{}
		return repositories.List, fmt.Errorf("fetching new repositories, count mismatch")
	}

	return repositories.List, nil
}

func FilterRepositoriesByAdmin(allRepos []repo) (filteredRepos []repo) {
	for _, repo := range allRepos {
		if repo.ViewerCanAdminister {
			filteredRepos = append(filteredRepos, repo)
		}
	}
	return
}
