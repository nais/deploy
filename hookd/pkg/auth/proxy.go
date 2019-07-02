package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	gh "github.com/google/go-github/v23/github"
	"github.com/shurcooL/githubv4"
	log "github.com/sirupsen/logrus"
)

type TeamsProxyHandler struct {
	ApplicationClient *gh.Client
}
type RepositoriesProxyHandler struct{}

func (h *TeamsProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	repository := q.Get("repository")
	accessToken, err := r.Cookie("accessToken")

	if err != nil || len(accessToken.Value) == 0 {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, err := getAuthenticatedUser(userClient(accessToken.Value))

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	filteredTeams, err := getFilteredTeams(h.ApplicationClient, repository, user.GetLogin())

	if err != nil {
		log.Error(err)
		filteredTeams = make([]*gh.Team, 0)
	}

	json, err := json.Marshal(filteredTeams)

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func (h *RepositoriesProxyHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("accessToken")

	if err != nil || len(accessToken.Value) == 0 {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	type repo struct {
		Name                githubv4.String `json:"name"`
		NameWithOwner       githubv4.String `json:"full_name"`
		ViewerCanAdminister githubv4.Boolean
	}

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

	var allRepos []repo
	for {
		err = graphqlClient(accessToken.Value).Query(context.Background(), &query, variables)
		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
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

	sort.Slice(allRepos, func(i, j int) bool {
		return allRepos[i].Name < allRepos[j].Name
	})

	json, err := json.Marshal(allRepos)

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}
