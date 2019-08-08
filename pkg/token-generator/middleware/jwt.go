package middleware

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/render"
	"github.com/navikt/deployment/pkg/token-generator/azure"
	"github.com/navikt/deployment/pkg/token-generator/httperr"
	"github.com/navikt/deployment/pkg/token-generator/session"
)

// Middleware that decodes a JWT token, validates it, and
// grants access only if it is valid.
func JWTMiddlewareHandler(certificates map[string]azure.CertificateList) middleware {

	// Validator function for the token. We pass the list of
	// certificates that can sign our tokens.
	validator := azure.JWTValidator(certificates)

	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {

			// Check if the user has a token in their session.
			sess := session.GetSession(r)
			if sess.Token == nil {
				render.Render(w, r, httperr.ErrUnauthorized(fmt.Errorf("no token")))
				return
			}

			// Parse and validate the JSON Web Token.
			token, err := jwt.ParseWithClaims(sess.Token.AccessToken, &sess.Claims, validator)

			if err != nil {
				render.Render(w, r, httperr.ErrUnauthorized(fmt.Errorf("invalid token: %s", err)))
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
