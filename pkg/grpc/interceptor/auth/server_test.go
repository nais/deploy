package auth_interceptor

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v2/jwt"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/pb"
	"google.golang.org/grpc/metadata"
)

func TestServerInterceptorApiKey(t *testing.T) {
	i := &ServerInterceptor{APIKeyStore: &mockAPIKeyStore{}}

	req := &pb.DeploymentRequest{
		Team: "team",
	}

	handler := func(ctx context.Context, req any) (any, error) {
		return nil, nil
	}

	t.Run("happy path", func(t *testing.T) {
		timestamp := time.Now().Format(time.RFC3339Nano)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{sign([]byte(timestamp), []byte("apikey"))},
			"timestamp":     []string{timestamp},
			"team":          []string{"team"},
		})

		_, err := i.UnaryServerInterceptor(ctx, req, nil, handler)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid apikey", func(t *testing.T) {
		timestamp := time.Now().Format(time.RFC3339Nano)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{sign([]byte(timestamp), []byte("invalid_apikey"))},
			"timestamp":     []string{timestamp},
			"team":          []string{"team"},
		})

		_, err := i.UnaryServerInterceptor(ctx, req, nil, handler)
		if err == nil {
			t.Fatal("got nil, want error")
		}

		want := "failed authentication"
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %s, want %s", err.Error(), want)
		}
	})

	t.Run("signature expired", func(t *testing.T) {
		timestamp := time.Now().Add((api_v1.MaxTimeSkew + 1) * time.Second).Format(time.RFC3339Nano)

		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"authorization": []string{sign([]byte(timestamp), []byte("apikey"))},
			"timestamp":     []string{timestamp},
			"team":          []string{"team"},
		})

		_, err := i.UnaryServerInterceptor(ctx, req, nil, handler)
		if err == nil {
			t.Fatal("got nil, want error")
		}

		want := "signature expired"
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %s, want %s", err.Error(), want)
		}
	})
}

func TestServerInterceptorJWT(t *testing.T) {
	i := &ServerInterceptor{
		APIKeyStore: &mockAPIKeyStore{},
		TokenValidator: &mockTokenValidator{
			repo:  "repo",
			valid: "valid",
		},
		TeamsClient: &mockTeamsClient{
			authorized: map[string]string{"repo": "team"},
		},
	}

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"jwt":  []string{"valid"},
		"team": []string{"team"},
	})

	t.Run("happy path", func(t *testing.T) {
		_, err := i.UnaryServerInterceptor(ctx, &pb.DeploymentRequest{}, nil, handler)
		if err != nil {
			t.Fatal(err)
		}
	})

	t.Run("invalid jwt", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"jwt":  []string{"invalid"},
			"team": []string{"team"},
		})

		_, err := i.UnaryServerInterceptor(ctx, &pb.DeploymentRequest{}, nil, handler)

		want := "invalid JWT token"
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %s, want suffix %s ", err.Error(), want)
		}
	})

	t.Run("missing team from metadata", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"jwt": []string{"valid"},
		})

		_, err := i.UnaryServerInterceptor(ctx, &pb.DeploymentRequest{}, nil, handler)

		want := "missing team in metadata"
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %s, want suffix %s ", err.Error(), want)
		}
	})

	t.Run("repo not authorized by team", func(t *testing.T) {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
			"jwt":  []string{"valid"},
			"team": []string{"wrong_team"},
		})

		_, err := i.UnaryServerInterceptor(ctx, &pb.DeploymentRequest{}, nil, handler)

		want := "repo \"repo\" not authorized by team \"wrong_team\""
		if !strings.HasSuffix(err.Error(), want) {
			t.Fatalf("got %s, want suffix %s ", err.Error(), want)
		}
	})
}

type mockAPIKeyStore struct{}

func (m *mockAPIKeyStore) ApiKeys(ctx context.Context, id string) (database.ApiKeys, error) {
	return database.ApiKeys{
		database.ApiKey{
			Key:     api_v1.Key("apikey"),
			Team:    "team",
			Expires: time.Now().Add(time.Duration(30 * time.Second)),
		},
	}, nil
}

func (m *mockAPIKeyStore) RotateApiKey(ctx context.Context, team string, key api_v1.Key) error {
	return nil
}

type mockTokenValidator struct {
	repo  string
	valid string
}

func (m *mockTokenValidator) Validate(ctx context.Context, token string) (jwt.Token, error) {
	if token != m.valid {
		return nil, fmt.Errorf("invalid token")
	}

	return jwt.NewBuilder().Claim("repository", m.repo).Build()
}

type mockTeamsClient struct {
	authorized map[string]string
}

func (m *mockTeamsClient) IsAuthorized(ctx context.Context, repo, team string) bool {
	return m.authorized[repo] == team
}

func handler(ctx context.Context, req any) (any, error) {
	return nil, nil
}
