package auth

import (
	"net/http"

	gh "github.com/google/go-github/v27/github"
	log "github.com/sirupsen/logrus"
)

type FormHandler struct{}

type FormData struct {
	User *gh.User
}

func (h *FormHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("accessToken")

	if err != nil || len(accessToken.Value) == 0 {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	user, err := getAuthenticatedUser(userClient(accessToken.Value))

	if err != nil {
		log.Error(err)
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	data := FormData{
		User: user,
	}

	page, err := templateWithBase("form.html")
	if err != nil {
		log.Errorf("error while parsing page templates: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = page.Execute(w, data); err != nil {
		log.Errorf("error while serving page: %s", err)
	}
}
