package github

import (
	"context"

	"github.com/prometheus/common/log"
	"github.com/shurcooL/githubv4"
)

type repo struct {
	Name                githubv4.String `json:"name"`
	NameWithOwner       githubv4.String `json:"full_name"`
	ViewerCanAdminister githubv4.Boolean
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
			if repo.ViewerCanAdminister {
				allRepos = append(allRepos, repo)
			}
		}

		if !query.Organization.Repositories.PageInfo.HasNextPage {
			break
		}
		variables["repositoriesCursor"] = githubv4.NewString(query.Organization.Repositories.PageInfo.EndCursor)
	}
	return
}
