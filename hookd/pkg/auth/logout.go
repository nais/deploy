package auth

import (
	"net/http"
)

type LogoutHandler struct{}

func (h *LogoutHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	for _, cookieName := range []string{"accessToken", "authState"} {
		http.SetCookie(w, &http.Cookie{
			Name:   cookieName,
			Value:  "",
			MaxAge: 0,
		})
	}

	http.Redirect(w, r, "/auth/login", http.StatusFound)
}
