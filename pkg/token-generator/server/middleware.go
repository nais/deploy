package server

import (
	"errors"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/render"
	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/navikt/deployment/pkg/token-generator/azure"
	"github.com/prometheus/common/log"
)

type middleware func(http.Handler) http.Handler

var (
	ErrInvalidBasicAuth = errors.New("invalid data in basic auth")
)

func ApiKeyMiddlewareHandler(source apikeys.Source) middleware {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			team, key, ok := r.BasicAuth()
			if !ok {
				render.Render(w, r, ErrForbidden(ErrInvalidBasicAuth))
				return
			}

			err := source.Validate(team, key)
			if err != nil {
				render.Render(w, r, ErrForbidden(err))
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

// Return a middleware that will redirect the client if not logged in via OAuth2.
func JWTMiddlewareHandler(certificates map[string]azure.CertificateList) middleware {
	validator := azure.JWTValidator(certificates)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			session := GetSession(r)
			if session.Token == nil {
				http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
				return
			}

			token, err := jwt.Parse(session.Token.AccessToken, validator)

			if err != nil {
				log.Infof("invalid token: %s", err)
				http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
				return
			}

			session.JWT = token
			session.Save()

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
