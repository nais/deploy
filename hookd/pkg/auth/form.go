package auth

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"

	gh "github.com/google/go-github/v23/github"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
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

func (h *FormHandler) isTeamMaintainer(login string, team *gh.Team) (bool, error) {
	membership, _, err := h.ApplicationClient.Teams.GetTeamMembership(context.Background(), team.GetID(), login)

	if err != nil {
		return false, nil
	}

	return membership.GetRole() == "maintainer", nil
}

func (h *FormHandler) getTeams(repository string) ([]*gh.Team, error) {
	opt := &gh.ListOptions{
		PerPage: 50,
	}

	var allTeams []*gh.Team

	for {
		teams, resp, err := h.ApplicationClient.Repositories.ListTeams(context.Background(), "navikt", repository, opt)

		if err != nil {
			return nil, fmt.Errorf("Could not fetch repository teams: %s", err)
		}

		allTeams = append(allTeams, teams...)

		if resp.NextPage == 0 {
			break
		}

		opt.Page = resp.NextPage
	}

	return allTeams, nil
}

func (h *FormHandler) filterTeams(teams []*gh.Team, login string) ([]*gh.Team, error) {
	var filteredTeams []*gh.Team

	for _, team := range teams {
		isMaintainer, err := h.isTeamMaintainer(login, team)

		if err != nil {
			return nil, fmt.Errorf("Error checking team role: %s", err)
		}

		if isMaintainer {
			filteredTeams = append(filteredTeams, team)
		}
	}

	return filteredTeams, nil
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

	h.userClient = gh.NewClient(tc)

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
		repositoryTeams, err := h.getTeams(repository)

		if err != nil {
			log.Error(err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		filteredTeams, err = h.filterTeams(repositoryTeams, user.GetLogin())

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
