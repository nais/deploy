package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"

	gh "github.com/google/go-github/v23/github"
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

	opt := &gh.RepositoryListByOrgOptions{
		ListOptions: gh.ListOptions{
			PerPage: 50,
		},
	}

	var allRepos []*gh.Repository

	for {
		repos, resp, err := userClient(accessToken.Value).Repositories.ListByOrg(context.Background(), "navikt", opt)

		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		for _, repo := range repos {
			if repo.GetPermissions()["admin"] {
				allRepos = append(allRepos, repo)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	sort.Slice(allRepos, func(i, j int) bool {
		return *allRepos[i].Name < *allRepos[j].Name
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
