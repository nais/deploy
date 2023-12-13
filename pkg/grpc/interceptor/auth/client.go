package auth_interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type ClientInterceptor struct {
	APIKey     []byte
	JWT        string
	RequireTLS bool
	Team       string
}

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}

func (c *ClientInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	if c.JWT != "" {
		return map[string]string{
			"jwt":  c.JWT,
			"team": c.Team,
		}, nil
	}

	timestamp := time.Now().Format(time.RFC3339Nano)
	return map[string]string{
		"authorization": sign([]byte(timestamp), c.APIKey),
		"timestamp":     timestamp,
		"team":          c.Team,
	}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}
