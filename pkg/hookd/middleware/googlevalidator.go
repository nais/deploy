package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/shurcooL/graphql"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
)

type TokenInfo struct {
	IssuedTo   string `json:"issued_to"`
	Audience   string `json:"audience"`
	UserId     string `json:"user_id"`
	Scope      string `json:"scope"`
	ExpiresIn  int    `json:"expires_in"`
	AccessType string `json:"access_type"`
	Email      string `json:"email"`
}

func GoogleValidatorMiddleware(audience string, apiKey string, consoleUrl string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if len(authHeader) < len("Bearer ") {
				http.Error(w, "Failed to authenticate", http.StatusUnauthorized)
				return // no token
			}
			token := authHeader[len("Bearer "):]

			req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fmt.Sprintf("https://www.googleapis.com/oauth2/v1/tokeninfo?access_token=%s", token), nil)
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				log.Warnf("Error creating http request: %v", err)
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				http.Error(w, "Internal error", http.StatusInternalServerError)
				log.Warnf("Error validating token: %v", err)
				return
			}
			defer resp.Body.Close()

			switch {
			case resp.StatusCode >= http.StatusInternalServerError:
				http.Error(w, "Unavailable", http.StatusServiceUnavailable)
				body, _ := io.ReadAll(resp.Body)
				log.Warnf("Google returned %s: %s", resp.Status, string(body))
				return
			case resp.StatusCode >= http.StatusBadRequest:
				http.Error(w, "Failed to authenticate", http.StatusUnauthorized)
				return
			case resp.StatusCode == http.StatusOK:
				break
			default:
				http.Error(w, "Internal error", http.StatusInternalServerError)
				body, _ := io.ReadAll(resp.Body)
				log.Warnf("Google returned %s: %s", resp.Status, string(body))
				return
			}

			tokenInfo, err := validateGoogleTokenInfo(resp.Body, audience)
			if err != nil {
				http.Error(w, "Failed to authenticate", http.StatusUnauthorized)
				log.Warnf("Error validating token: %v", err)
				return
			}

			groups, err := getGroupsFromConsole(r.Context(), tokenInfo.Email, apiKey, consoleUrl)
			if err != nil {
				http.Error(w, "Unavailable", http.StatusServiceUnavailable)
				log.Warnf("Error resolving groups: %v", err)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), "groups", groups))
			next.ServeHTTP(w, r)
		}
		return http.HandlerFunc(fn)
	}
}

func getGroupsFromConsole(ctx context.Context, id string, key string, url string) ([]string, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: key},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := graphql.NewClient(url, httpClient)

	type UsersQuery struct {
		email string
	}
	var q struct {
		Users struct {
			Nodes []struct {
				Email graphql.String
				Teams []struct {
					Slug graphql.String
				}
			}
		} `graphql:"users(query: $query)"`
	}
	variables := map[string]interface{}{
		"query": UsersQuery{email: id},
	}
	err := client.Query(ctx, &q, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups from console: %w", err)
	}

	var groups []string
	for _, node := range q.Users.Nodes {
		for _, team := range node.Teams {
			groups = append(groups, string(team.Slug))
		}
	}
	return groups, nil
}

func validateGoogleTokenInfo(body io.ReadCloser, audience string) (*TokenInfo, error) {
	var tokenInfo TokenInfo
	err := json.NewDecoder(body).Decode(&tokenInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to decode tokenInfo: %w", err)
	}

	if tokenInfo.ExpiresIn <= 0 {
		return nil, fmt.Errorf("token expired")
	}

	if tokenInfo.Audience != audience {
		return nil, fmt.Errorf("incorrect audience")
	}

	return &tokenInfo, nil
}
