package auth

import (
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type CallbackHandler struct {
	ClientID     string
	ClientSecret string
}

func (h *CallbackHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if len(code) == 0 || len(state) == 0 {
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return
	}

	cookieState, err := r.Cookie("authState")

	if err != nil || state != cookieState.Value {
		http.Redirect(w, r, "/auth/logout", http.StatusFound)
		return
	}

	queryParams := url.Values{
		"client_id":     []string{h.ClientID},
		"client_secret": []string{h.ClientSecret},
		"code":          []string{code},
	}.Encode()

	response, err := http.Post(
		"https://github.com/login/oauth/access_token?"+queryParams,
		"text/plain",
		strings.NewReader(""),
	)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseBody, err := ioutil.ReadAll(response.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	parsedBody, err := url.ParseQuery(string(responseBody))

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	error := parsedBody.Get("error")

	if len(error) != 0 {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	accessToken := parsedBody.Get("access_token")

	cookie := http.Cookie{
		Name:    "accessToken",
		Value:   accessToken,
		Expires: time.Now().Add(1 * time.Hour),
		Path:    "/",
	}
	http.SetCookie(w, &cookie)
	http.Redirect(w, r, "/auth/form", http.StatusFound)
}
