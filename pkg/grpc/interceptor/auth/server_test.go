package auth_interceptor

import (
	"context"
	"strings"
	"testing"
	"time"

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

		if !strings.HasSuffix(err.Error(), "HMAC signature error") {
			t.Fatalf("got %s, want HMAC signature error", err.Error())
		}
	})

	t.Run("signature too old", func(t *testing.T) {
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

		if !strings.HasSuffix(err.Error(), "signature is too old") {
			t.Fatalf("got %s, want signature is too old", err.Error())
		}
	})
}

// func TestServerInterceptorJWT(t *testing.T) {
// 	i := &ServerInterceptor{APIKeyStore: &mockAPIKeyStore{}}
//
// 	req := &pb.DeploymentRequest{
// 		Team: "team",
// 	}
//
// 	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
// 		"authorization": []string{"Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ0ZWFtIjoidGVhbSIsInRpbWVzdGFtcCI6IjIwMjAtMDctMjBUMTc6MjE6MjAuMjA0WiIsInRlYW0iOiJ0ZWFtIn0.1WZ7M4U3BQ3zXWYkU8J7JXVQ5h3Uw5h2c0q5Q8kKs8k"},
// 	})
//
// 	handler := func(ctx context.Context, req any) (any, error) {
// 		return nil, nil
// 	}
//
// 	_, err := i.UnaryServerInterceptor(ctx, req, nil, handler)
// 	if err != nil {
// 		t.Fatal(err)
// 	}
// }

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
