package auth

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	gh "github.com/google/go-github/v23/github"
	log "github.com/sirupsen/logrus"
)

type FormHandler struct {
	accessToken       string
	userClient        *gh.Client
	ApplicationClient *gh.Client
}

type FormData struct {
	Repositories []*gh.Repository
	Teams        []*gh.Team
	Repository   string
	User         *gh.User
}

func (h *FormHandler) getRepositories() ([]*gh.Repository, error) {
	opt := &gh.RepositoryListByOrgOptions{
		ListOptions: gh.ListOptions{
			PerPage: 50,
		},
	}

	var allRepos []*gh.Repository

	for {
		repos, resp, err := h.userClient.Repositories.ListByOrg(context.Background(), "navikt", opt)

		if err != nil {
			return nil, fmt.Errorf("Could not fetch repositories: %s", err)
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

	return allRepos, nil
}

func (h *FormHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("accessToken")

	if err != nil || len(accessToken.Value) == 0 {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	h.userClient = userClient(accessToken.Value)

	user, _, err := h.userClient.Users.Get(context.Background(), "")

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	repository := r.URL.Query().Get("repository")
	var repositories []*gh.Repository
	var filteredTeams []*gh.Team

	if len(repository) != 0 {
		repositoryTeams, err := getTeams(h.ApplicationClient, repository)

		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		filteredTeams, err = filterTeams(h.ApplicationClient, repositoryTeams, user.GetLogin())

		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	} else {
		repositories, err = h.getRepositories()

		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	data := FormData{
		Repositories: repositories,
		Teams:        filteredTeams,
		Repository:   repository,
		User:         user,
	}

	page, err := template.ParseFiles(
		filepath.Join(TemplateLocation, "site.html"),
		filepath.Join(TemplateLocation, "form.html"),
	)

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	page.Execute(w, data)
}
