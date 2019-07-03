package auth

import (
	"net/http"
	"strings"

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

	var repositoryName string

	fullName := r.Form.Get("repository")
	teamNames := r.Form["team[]"]

	arr := strings.Split(fullName, "/")
	if len(arr) != 2 {
		log.Warnf("while parsing the fullname of the repo '%s': %s", fullName, err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	repositoryName = arr[1]

	log.Tracef("Request from Github user '%s' that repository '%s' is granted access to the following teams: %+v", user.GetLogin(), fullName, teamNames)

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

	err = h.TeamRepositoryStorage.Write(fullName, teamNames)
	if err != nil {
		log.Error(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	log.Infof("The repository '%s' has been granted deployment access by Github user '%s' for the following teams: %+v", fullName, user.GetLogin(), teamNames)

	data := SubmittedFormData{
		Teams:      teamNames,
		Repository: repositoryName,
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
