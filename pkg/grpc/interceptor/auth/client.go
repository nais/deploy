package auth_interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

type ClientInterceptor struct {
	RequireTLS bool
	APIKey     []byte
	OIDCToken  string
	Team       string
}

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}

func (t *ClientInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	timestamp := time.Now().Format(time.RFC3339Nano)
	var authHeader string
	if t.ShouldUseOidc() {
		authHeader = fmt.Sprintf("Bearer %s", t.OIDCToken)
	} else {
		authHeader = sign([]byte(timestamp), t.APIKey)
	}
	return map[string]string{
		"authorization": authHeader,
		"timestamp":     timestamp,
		"team":          t.Team,
	}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}

func (t *ClientInterceptor) ShouldUseOidc() bool {
	return len(t.OIDCToken) != 0
}
