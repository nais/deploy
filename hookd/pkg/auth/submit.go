package auth

import (
	"net/http"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/hookd/pkg/database"
	log "github.com/sirupsen/logrus"
)

type SubmittedFormHandler struct {
	accessToken           string
	userClient            *gh.Client
	ApplicationClient     *gh.Client
	TeamRepositoryStorage database.RepositoryTeamStore
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

	user, err := getAuthenticatedUser(h.userClient)

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

	fullName := r.Form.Get("repository")
	teamNames := r.Form["team[]"]

	log.Tracef("Request from Github user '%s' that repository '%s' is granted access to the following teams: %+v", user.GetLogin(), fullName, teamNames)

	// retrieve the list of teams administered by the current user
	teams, err := getFilteredTeams(h.ApplicationClient, fullName, user.GetLogin())
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

	err = h.TeamRepositoryStorage.WriteRepositoryTeams(fullName, teamNames)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Infof("The repository '%s' has been granted deployment access by Github user '%s' for the following teams: %+v", fullName, user.GetLogin(), teamNames)

	data := SubmittedFormData{
		Teams:      teamNames,
		Repository: fullName,
		User:       user,
	}

	page, err := templateWithBase("submit.html")
	if err != nil {
		log.Errorf("error while parsing page templates: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = page.Execute(w, data); err != nil {
		log.Errorf("error while serving page: %s", err)
	}
}
