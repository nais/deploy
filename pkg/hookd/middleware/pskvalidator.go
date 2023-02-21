package middleware

import (
	"fmt"
	"net/http"
)

func PskValidatorMiddleware(keys []string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			psk := r.Header.Get("X-PSK")
			for _, key := range keys {
				if key == psk {
					next.ServeHTTP(w, r)
					return
				}
			}
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprintf(w, "Unauthorized access: Invalid key")
		}
		return http.HandlerFunc(fn)
	}
}
