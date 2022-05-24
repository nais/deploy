package middleware

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type TokenInfo struct {
	IssuedTo   string `json:"issued_to"`
	Audience   string `json:"audience"`
	UserId     string `json:"user_id"`
	Scope      string `json:"scope"`
	ExpiresIn  int    `json:"expires_in"`
	AccessType string `json:"access_type"`
}

func GoogleValidatorMiddleware(audience string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < len("Bearer ") {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Failed to authenticate")
				return // no token
			}
			token := authHeader[len("Bearer "):]

			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=%s", token), nil)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error creating http request: %s", err)
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error validating token: %s", err)
				return
			}
			defer resp.Body.Close()

			switch {
			case resp.StatusCode >= http.StatusInternalServerError:
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintf(w, "Unavailable")
				body, _ := io.ReadAll(resp.Body)
				log.Warnf("Google returned %s: %s", resp.Status, string(body))
				return
			case resp.StatusCode >= http.StatusBadRequest:
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Failed to authenticate")
				return
			case resp.StatusCode == http.StatusOK:
				break
			default:
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Unknown response")
				body, _ := io.ReadAll(resp.Body)
				log.Warnf("Google returned %s: %s", resp.Status, string(body))
				return
			}

			err = validateGoogleTokenInfo(resp.Body, audience)
			if err != nil {
				w.WriteHeader(http.StatusUnauthorized)
				fmt.Fprintf(w, "Failed to authenticate")
				log.Warnf("Error validating token: %v", err)
				return
			}
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func validateGoogleTokenInfo(body io.ReadCloser, audience string) error {
	var tokenInfo TokenInfo
	err := json.NewDecoder(body).Decode(&tokenInfo)
	if err != nil {
		return fmt.Errorf("failed to decode tokenInfo: %w", err)
	}

	if tokenInfo.ExpiresIn <= 0 {
		return fmt.Errorf("token expired")
	}

	if tokenInfo.Audience != audience {
		return fmt.Errorf("incorrect audience")
	}

	return nil
}
