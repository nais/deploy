package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-chi/jwtauth"
	"github.com/navikt/deployment/hookd/pkg/azure/discovery"
	"github.com/navikt/deployment/hookd/pkg/azure/validate"
)

func TokenValidatorMiddleware(certificates map[string]discovery.CertificateList) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			var claims jwt.MapClaims

			token := jwtauth.TokenFromHeader(r)

			_, err := jwt.ParseWithClaims(token, &claims, validate.JWTValidator(certificates))
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, "Unauthorized access: %s", err.Error())
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), "claims", claims))
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
