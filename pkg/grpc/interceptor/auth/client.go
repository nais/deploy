package auth_interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type ClientInterceptor struct {
	RequireTLS bool
	APIKey     []byte
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
	return map[string]string{
		"authorization": sign([]byte(timestamp), t.APIKey),
		"timestamp":     timestamp,
		"team":          t.Team,
		"jwt":           "eySomething",
	}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}
