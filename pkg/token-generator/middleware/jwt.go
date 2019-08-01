package middleware

import (
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/navikt/deployment/pkg/token-generator/azure"
	"github.com/navikt/deployment/pkg/token-generator/session"
	log "github.com/sirupsen/logrus"
)

const (
	authorizeEndpoint = "/auth/login"
)

// Return a middleware that will redirect the client if not logged in via OAuth2.
func JWTMiddlewareHandler(certificates map[string]azure.CertificateList) middleware {

	// Validator function for the token. We pass the list of
	// certificates that can sign our tokens.
	validator := azure.JWTValidator(certificates)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {

			// If the client doesn't have a session cookie, they must log in.
			sess := session.GetSession(r)
			if sess.Token == nil {
				http.Redirect(w, r, authorizeEndpoint, http.StatusTemporaryRedirect)
				return
			}

			// Parse and validate the JSON Web Token.
			// Any errors and the client must log in again.
			token, err := jwt.Parse(sess.Token.AccessToken, validator)

			if err != nil {
				log.Infof("invalid token: %s", err)
				http.Redirect(w, r, authorizeEndpoint, http.StatusTemporaryRedirect)
				return
			}

			// The token has been signed by our trusted authority.
			// Persist the session to memory so that it can be used
			// by request handlers.
			sess.JWT = token
			sess.Save()

			// Pass the request down the chain.
			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}
