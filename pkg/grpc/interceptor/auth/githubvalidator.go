package auth_interceptor

import (
	"context"
	"fmt"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwk"
	"github.com/lestrrat-go/jwx/v2/jws"
	"github.com/lestrrat-go/jwx/v2/jwt"
)

const (
	Audience               = "hookd"
	GithubOIDCDiscoveryURL = "https://token.actions.githubusercontent.com/.well-known/jwks"
	Issuer                 = "https://token.actions.githubusercontent.com"
)

type GithubValidator struct {
	jwkCache *jwk.Cache
}

func NewGithubValidator() (*GithubValidator, error) {
	g := &GithubValidator{}
	if err := g.setupJwkAutoRefresh(); err != nil {
		return nil, fmt.Errorf("setup jwk auto refresh: %w", err)
	}
	return g, nil
}

func (g *GithubValidator) Validate(ctx context.Context, token string) (jwt.Token, error) {
	pubKeys, err := g.jwkCache.Get(ctx, GithubOIDCDiscoveryURL)
	if err != nil {
		panic(fmt.Errorf("get jwk from cache: %w", err))
	}
	keySetOpts := jwt.WithKeySet(pubKeys, jws.WithInferAlgorithmFromKey(true))
	otherParseOpts := g.jwtOptions()
	t, err := jwt.Parse([]byte(token), append(otherParseOpts, keySetOpts)...)
	if err != nil {
		return nil, fmt.Errorf("invalid JWT token: %w", err)
	}

	return t, nil
}

func (g *GithubValidator) jwtOptions() []jwt.ParseOption {
	return []jwt.ParseOption{
		jwt.WithValidate(true),
		jwt.WithAcceptableSkew(5 * time.Second),
		jwt.WithIssuer(Issuer),
		jwt.WithAudience(Audience),
	}
}

func (g *GithubValidator) setupJwkAutoRefresh() error {
	ctx := context.Background()

	cache := jwk.NewCache(ctx)
	err := cache.Register(GithubOIDCDiscoveryURL, jwk.WithRefreshInterval(time.Hour))
	if err != nil {
		return fmt.Errorf("jwks caching: %w", err)
	}
	// force initial refresh
	_, err = cache.Refresh(ctx, GithubOIDCDiscoveryURL)
	if err != nil {
		return fmt.Errorf("jwks caching: %w", err)
	}
	g.jwkCache = cache

	_, err = g.jwkCache.Get(context.Background(), GithubOIDCDiscoveryURL)
	if err != nil {
		return fmt.Errorf("get jwk from cache: %w", err)
	}

	return nil
}
