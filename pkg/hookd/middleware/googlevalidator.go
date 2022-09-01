package middleware

import (
	"context"
	"fmt"
	"github.com/shurcooL/graphql"
	log "github.com/sirupsen/logrus"
	"golang.org/x/oauth2"
	"net/http"
	"strings"
	"time"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/lestrrat-go/jwx/jwt"
)

const (
	googleDiscoveryURL = "https://www.googleapis.com/oauth2/v3/certs"
	googleIssuer       = "https://accounts.google.com"
)

type GoogleValidator struct {
	clientID       string
	consoleApiKey  string
	consoleUrl     string
	allowedDomains []string
	jwkAutoRefresh *jwk.AutoRefresh
}

func NewGoogleValidator(clientID, consoleApiKey, consoleUrl string, allowedDomains []string) (*GoogleValidator, error) {
	google := GoogleValidator{
		clientID:       clientID,
		consoleApiKey:  consoleApiKey,
		consoleUrl:     consoleUrl,
		allowedDomains: allowedDomains,
	}
	err := google.setupJwkAutoRefresh()
	if err != nil {
		return nil, err
	}

	return &google, nil
}

func (g *GoogleValidator) setupJwkAutoRefresh() error {
	ctx := context.Background()

	ar := jwk.NewAutoRefresh(ctx)
	ar.Configure(googleDiscoveryURL, jwk.WithMinRefreshInterval(time.Hour))

	// trigger initial token fetch
	_, err := ar.Refresh(ctx, googleDiscoveryURL)
	if err != nil {
		return fmt.Errorf("fetch jwks: %w", err)
	}

	g.jwkAutoRefresh = ar
	return nil
}

func (g *GoogleValidator) KeySetFrom(t jwt.Token) (jwk.Set, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return g.jwkAutoRefresh.Fetch(ctx, googleDiscoveryURL)
}

func (g *GoogleValidator) jwtOptions() []jwt.ParseOption {
	return []jwt.ParseOption{
		jwt.WithValidate(true),
		jwt.InferAlgorithmFromKey(true),
		jwt.WithAcceptableSkew(5 * time.Second),
		jwt.WithIssuer(googleIssuer),
		jwt.WithKeySetProvider(g),
		jwt.WithAudience(g.clientID),
		jwt.WithRequiredClaim("email"),
		jwt.WithRequiredClaim("hd"),
	}
}

func (g *GoogleValidator) Middleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			bearer := strings.TrimSpace(r.Header.Get("Authorization"))
			token := strings.TrimSpace(strings.TrimPrefix(bearer, "Bearer"))

			email, err := g.parseAndValidateToken(token)
			if err != nil {
				http.Error(w, "Failed to authenticate", http.StatusUnauthorized)
				log.Warnf("Error parsing and validating token: %v", err)
				return
			}

			groups, err := getGroupsFromConsole(r.Context(), email, g.consoleApiKey, g.consoleUrl)
			if err != nil {
				http.Error(w, "Unavailable", http.StatusServiceUnavailable)
				log.Warnf("Error resolving groups: %v", err)
				return
			}

			r = r.WithContext(WithEmail(r.Context(), email))
			r = r.WithContext(WithGroups(r.Context(), groups))
			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

func (g *GoogleValidator) parseAndValidateToken(token string) (string, error) {
	tok, err := jwt.ParseString(token, g.jwtOptions()...)
	if err != nil {
		return "", fmt.Errorf("parse token: %w", err)
	}

	emailClaim, _ := tok.Get("email")
	email, _ := emailClaim.(string)
	if email == "" {
		return "", fmt.Errorf("empty email claim in token")
	}

	subClaim, _ := tok.Get("sub")
	sub, _ := subClaim.(string)
	if sub == "" {
		return "", fmt.Errorf("empty sub claim in token")
	}

	if len(g.allowedDomains) > 0 {
		hdClaim, _ := tok.Get("hd")
		hd, _ := hdClaim.(string)

		found := false
		for _, allowedDomain := range g.allowedDomains {
			if hd == allowedDomain {
				found = true
				break
			}
		}

		if !found {
			return "", fmt.Errorf("'%s' not in allowed domains: %v", hd, g.allowedDomains)
		}
	}

	return email, nil
}

func getGroupsFromConsole(ctx context.Context, id string, key string, url string) ([]string, error) {
	src := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: key},
	)
	httpClient := oauth2.NewClient(ctx, src)
	client := graphql.NewClient(url, httpClient)

	var q struct {
		User struct {
			Teams []struct {
				Team struct {
					Slug graphql.String
				}
			}
		} `graphql:"userByEmail(email: $query)"`
	}
	variables := map[string]interface{}{
		"query": graphql.String(id),
	}
	err := client.Query(ctx, &q, variables)
	if err != nil {
		return nil, fmt.Errorf("failed to get groups from console: %w", err)
	}

	var groups []string
	for _, team := range q.User.Teams {
		groups = append(groups, string(team.Team.Slug))
	}
	return groups, nil
}
