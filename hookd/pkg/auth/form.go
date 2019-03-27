package auth

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	gh "github.com/google/go-github/v23/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type FormHandler struct {
	accessToken string
	apiClient   *gh.Client
}

type FormData struct {
	Repositories []*gh.Repository
	Teams        []*gh.Team
	User         *gh.User
}

func (h *FormHandler) getRepositories() ([]*gh.Repository, error) {
	opt := &gh.RepositoryListByOrgOptions{
		ListOptions: gh.ListOptions{
			PerPage: 50,
		},
		Type: "member",
	}

	var allRepos []*gh.Repository

	for {
		repos, resp, err := h.apiClient.Repositories.ListByOrg(context.Background(), "navikt", opt)

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

func (h *FormHandler) getTeams() ([]*gh.Team, error) {
	opt := &gh.ListOptions{PerPage: 50000}

	var allTeams []*gh.Team

	for {
		teams, resp, err := h.apiClient.Teams.ListTeams(context.Background(), "navikt", opt)

		if err != nil {
			return nil, errors.New("Could not fetch teams")
		}

		for _, team := range teams {
			if team.GetPermission() != "admin" {
				allTeams = append(allTeams, team)
			}
		}

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allTeams, nil
}

func (h *FormHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("accessToken")

	if err != nil || len(accessToken.Value) == 0 {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	h.accessToken = accessToken.Value

	ts := oauth2.StaticTokenSource(&oauth2.Token{
		AccessToken: h.accessToken,
	})

	tc := oauth2.NewClient(context.Background(), ts)

	h.apiClient = gh.NewClient(tc)

	repositories, err := h.getRepositories()

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	teams, err := h.getTeams()

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	user, _, err := h.apiClient.Users.Get(context.Background(), "")

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := FormData{
		Repositories: repositories,
		Teams:        teams,
		User:         user,
	}

	page, err := template.ParseFiles(
		filepath.Join(TemplateLocation, "site.html"),
		filepath.Join(TemplateLocation, "form.html"),
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	page.Execute(w, data)
}
