package auth

import (
	"context"
	"html/template"
	"net/http"
	"path/filepath"

	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

type SubmittedFormHandler struct {
	accessToken           string
	userClient            *gh.Client
	ApplicationClient     *gh.Client
	TeamRepositoryStorage persistence.TeamRepositoryStorage
}

type SubmittedFormData struct {
	Teams      []string
	Repository string
	User       *gh.User
}

func (h *SubmittedFormHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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

	err = r.ParseForm()
	if err != nil {
		log.Warnf("while parsing form data: %s", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	repositoryName := r.Form.Get("repository")
	teamNames := r.Form["teams"]

	// retrieve the list of teams administered by the current user
	teams, err := getFilteredTeams(h.ApplicationClient, repositoryName, user.GetLogin())
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// check that the user submitted only teams that they can administer
	err = teamListsMatch(teamNames, teams)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusForbidden)
		return
	}

	err = h.TeamRepositoryStorage.Write(repositoryName, teamNames)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	data := SubmittedFormData{
		Teams:      teamNames,
		Repository: repositoryName,
		User:       user,
	}

	page, err := template.ParseFiles(
		filepath.Join(TemplateLocation, "site.html"),
		filepath.Join(TemplateLocation, "submit.html"),
	)

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	page.Execute(w, data)
}
