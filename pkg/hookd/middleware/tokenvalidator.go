package middleware

import (
	"context"
	"fmt"
	"net/http"

	"github.com/golang-jwt/jwt"
	"github.com/go-chi/jwtauth"
	"github.com/nais/deploy/pkg/azure/discovery"
	"github.com/nais/deploy/pkg/azure/validate"
)

func TokenValidatorMiddleware(certificates map[string]discovery.CertificateList, audience string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			var claims jwt.MapClaims

			token := jwtauth.TokenFromHeader(r)

			_, err := jwt.ParseWithClaims(token, &claims, validate.JWTValidator(certificates, audience))
			if err != nil {
				w.WriteHeader(http.StatusForbidden)
				fmt.Fprintf(w, "Unauthorized access: %s", err.Error())
				return
			}

			var groups []string
			groupInterface := claims["groups"].([]interface{})
			groups = make([]string, len(groupInterface))
			for i, v := range groupInterface {
				groups[i] = v.(string)
			}
			r = r.WithContext(context.WithValue(r.Context(), "claims", claims))
			r = r.WithContext(context.WithValue(r.Context(), "groups", groups))
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
