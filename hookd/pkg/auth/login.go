package auth

import (
	"net/http"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

type LoginHandler struct {
	ClientID string
}

func (h *LoginHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	accessToken, err := r.Cookie("accessToken")

	if err == nil && len(accessToken.Value) != 0 {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	state, err := uuid.NewRandom()

	if err != nil {
		log.Errorf("error in UUID generator: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	page, err := templateWithBase("login.html")
	if err != nil {
		log.Errorf("error while parsing page templates: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cookie := http.Cookie{
		Name:    "authState",
		Value:   state.String(),
		Expires: time.Now().Add(10 * time.Minute),
		Path:    "/",
	}
	http.SetCookie(w, &cookie)

	data := PageData{
		ClientID: h.ClientID,
		State:    state.String(),
	}
	if err = page.Execute(w, data); err != nil {
		log.Errorf("error while serving page: %s", err)
	}
}
