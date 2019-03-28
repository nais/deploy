package auth

import (
	"html/template"
	"net/http"
	"path/filepath"
	"time"

	"github.com/google/uuid"
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	page, err := template.ParseFiles(
		filepath.Join(TemplateLocation, "site.html"),
		filepath.Join(TemplateLocation, "login.html"),
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	cookie := http.Cookie{
		Name:    "authState",
		Value:   state.String(),
		Expires: time.Now().Add(10 * time.Minute),
	}
	http.SetCookie(w, &cookie)

	page.Execute(w, PageData{
		ClientID: h.ClientID,
		State:    state.String(),
	})
}
