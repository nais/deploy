package auth

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/github"
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

	allRepos, err := github.GetRepositories(graphqlClient(accessToken.Value))
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	sort.Slice(allRepos, func(i, j int) bool {
		return strings.ToLower(allRepos[i].Name) < strings.ToLower(allRepos[j].Name)
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
