package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

type authHandler struct {
	authCodeOption oauth2.AuthCodeOption

	config oauth2.Config
}

var (
	azureAuthorizeURL = "https://login.microsoftonline.com/%s/oauth2/%s"

	ErrStateNoMatch = errors.New("the 'state' parameter doesn't match, maybe you are a victim of cross-site request forgery")
)

func NewAuthHandler(clientID, clientSecret, tenant, objectID, redirectURL, resource string) *authHandler {
	handler := &authHandler{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:   fmt.Sprintf(azureAuthorizeURL, tenant, "authorize"),
				TokenURL:  fmt.Sprintf(azureAuthorizeURL, tenant, "token"),
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
	}

	handler.authCodeOption = oauth2.SetAuthURLParam("resource", resource)

	return handler
}

// Authorize redirects a client to sign in with their Microsoft account
func (h *authHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	session.State = uuid.New().String()
	session.Save()

	authorizeURL := h.config.AuthCodeURL(session.State, h.authCodeOption)

	http.SetCookie(w, session.Cookie())
	http.Redirect(w, r, authorizeURL, http.StatusTemporaryRedirect)
}

// Callback is called when Microsoft has authorized the user.
// Note that the token that is stored in the session must be further validated
// before trusting the client.
func (h *authHandler) Callback(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)

	if session.State != r.FormValue("state") {
		render.Render(w, r, ErrInvalidRequest(ErrStateNoMatch))
		return
	}

	token, err := h.config.Exchange(r.Context(), r.FormValue("code"))
	if err != nil {
		render.Render(w, r, ErrForbidden(err))
		return
	}

	if !token.Valid() {
		render.Render(w, r, ErrForbidden(fmt.Errorf("invalid token")))
		return
	}

	session.Token = token
	session.Save()
	http.SetCookie(w, session.Cookie())

	if err != nil {
		render.Render(w, r, ErrUnavailable(err))
		return
	}

	http.Redirect(w, r, "/auth/echo", http.StatusTemporaryRedirect)
}

// Echo is a debug function
func (h *authHandler) Echo(w http.ResponseWriter, r *http.Request) {
	session := GetSession(r)
	render.JSON(w, r, session)
}
