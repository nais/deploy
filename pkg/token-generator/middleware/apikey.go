package middleware

import (
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/navikt/deployment/pkg/token-generator/httperr"
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
				render.Render(w, r, httperr.ErrForbidden(ErrInvalidBasicAuth))
				return
			}

			err := source.Validate(team, key)
			if err != nil {
				render.Render(w, r, httperr.ErrForbidden(err))
				return
			}

			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}
