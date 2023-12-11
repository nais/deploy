package auth_interceptor_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	auth_interceptor "github.com/nais/deploy/pkg/grpc/interceptor/auth"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/pb"
	"google.golang.org/grpc/metadata"
)

func TestServerInterceptor(t *testing.T) {
	i := &auth_interceptor.ServerInterceptor{APIKeyStore: &mockAPIKeyStore{}}

	req := &pb.DeploymentRequest{
		Team: "team",
	}

	timestamp := time.Now().Format(time.RFC3339Nano)

	ctx := metadata.NewIncomingContext(context.Background(), metadata.MD{
		"authorization": []string{sign([]byte(timestamp), []byte("apikey"))},
		"timestamp":     []string{timestamp},
		"team":          []string{"team"},
		"jwt":           []string{"jwt"},
	})

	resp, err := i.UnaryServerInterceptor(ctx, req, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(resp)
}

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}

type mockAPIKeyStore struct{}

func (m *mockAPIKeyStore) ApiKeys(ctx context.Context, id string) (database.ApiKeys, error) {
	return database.ApiKeys{
		database.ApiKey{
			Key:  api_v1.Key("apikey"),
			Team: "team",
		},
	}, nil
}

func (m *mockAPIKeyStore) RotateApiKey(ctx context.Context, team string, key api_v1.Key) error {
	return nil
}
