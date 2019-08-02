package server

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/token-generator/httperr"
	"github.com/navikt/deployment/pkg/token-generator/session"
	"golang.org/x/oauth2"
)

type authHandler struct {
	authCodeOption oauth2.AuthCodeOption

	config oauth2.Config
}

var (
	ErrStateNoMatch = errors.New("the 'state' parameter doesn't match, maybe you are a victim of cross-site request forgery")
)

func azureAuthorizeURL(tenant, endpoint string) string {
	return fmt.Sprintf("https://login.microsoftonline.com/%s/oauth2/%s", tenant, endpoint)
}

func NewAuthHandler(clientID, clientSecret, tenant, redirectURL, resource string) *authHandler {
	handler := &authHandler{
		config: oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:   azureAuthorizeURL(tenant, "authorize"),
				TokenURL:  azureAuthorizeURL(tenant, "token"),
				AuthStyle: oauth2.AuthStyleInParams,
			},
		},
	}

	handler.authCodeOption = oauth2.SetAuthURLParam("resource", resource)

	return handler
}

// Authorize redirects a client to sign in with their Microsoft account
func (h *authHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	session := session.GetSession(r)
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
	session := session.GetSession(r)

	if session.State != r.FormValue("state") {
		render.Render(w, r, httperr.ErrInvalidRequest(ErrStateNoMatch))
		return
	}

	token, err := h.config.Exchange(r.Context(), r.FormValue("code"))
	if err != nil {
		render.Render(w, r, httperr.ErrForbidden(err))
		return
	}

	if !token.Valid() {
		render.Render(w, r, httperr.ErrForbidden(fmt.Errorf("invalid token")))
		return
	}

	session.Token = token
	session.Save()
	http.SetCookie(w, session.Cookie())

	if err != nil {
		render.Render(w, r, httperr.ErrUnavailable(err))
		return
	}

	http.Redirect(w, r, "/auth/echo", http.StatusTemporaryRedirect)
}

// Echo is a debug function
func (h *authHandler) Echo(w http.ResponseWriter, r *http.Request) {
	session := session.GetSession(r)
	if session.Token == nil {
		http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
		return
	}
	render.JSON(w, r, session)
}
