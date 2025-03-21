package auth_interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
)

type ClientInterceptor interface {
	GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error)
	RequireTransportSecurity() bool
}

var _ ClientInterceptor = &APIKeyInterceptor{}

type APIKeyInterceptor struct {
	APIKey     []byte
	RequireTLS bool
	Team       string
}

func (c *APIKeyInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	timestamp := time.Now().Format(time.RFC3339Nano)
	return map[string]string{
		"authorization": sign([]byte(timestamp), c.APIKey),
		"timestamp":     timestamp,
		"team":          c.Team,
	}, nil
}

func (t *APIKeyInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}

var _ ClientInterceptor = &JWTInterceptor{}

type JWTInterceptor struct {
	JWT        string
	RequireTLS bool
	Team       string
}

func (c *JWTInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"jwt":  c.JWT,
		"team": c.Team,
	}, nil
}

func (t *JWTInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}

type GitHubTokenInterceptor struct {
	BearerToken string
	RequireTLS  bool
	TokenURL    string
	Team        string

	token          string
	tokenExpiresAt time.Time
	mu             sync.Mutex
}

func (g *GitHubTokenInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	token, err := g.Token(ctx)
	if err != nil {
		return nil, fmt.Errorf("getting GitHub JWT: %w", err)
	}

	return map[string]string{
		"jwt":  token,
		"team": g.Team,
	}, nil
}

func (g *GitHubTokenInterceptor) RequireTransportSecurity() bool {
	return g.RequireTLS
}

func (g *GitHubTokenInterceptor) Token(ctx context.Context) (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	const renewBefore = 1 * time.Minute
	shouldRenew := g.tokenExpiresAt.IsZero() || time.Now().After(g.tokenExpiresAt.Add(-renewBefore))
	if g.token != "" && !shouldRenew {
		return g.token, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, g.TokenURL, nil)
	if err != nil {
		return "", fmt.Errorf("creating request: %w", err)
	}
	q := req.URL.Query()
	q.Add("audience", "hookd")
	req.URL.RawQuery = q.Encode()
	req.Header.Set("Authorization", "bearer "+g.BearerToken)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching token: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading body: %w", err)
	}

	var tokenResponse struct {
		Token string `json:"value"`
	}
	err = json.Unmarshal(body, &tokenResponse)
	if err != nil {
		return "", fmt.Errorf("unmarshalling json: %w", err)
	}

	// Skip signature verification; we only care about the expiration time here.
	// The receiving party (i.e., server) must verify the token anyway.
	j, err := jwt.ParseString(tokenResponse.Token,
		jwt.WithVerify(false),
	)
	if err != nil {
		return "", fmt.Errorf("parsing JWT: %w", err)
	}

	g.token = tokenResponse.Token
	g.tokenExpiresAt = j.Expiration()
	return tokenResponse.Token, nil
}

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}
