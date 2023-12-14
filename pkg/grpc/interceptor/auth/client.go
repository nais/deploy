package auth_interceptor

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type ClientInterceptor interface {
	GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error)
	RequireTransportSecurity() bool
}

var _ ClientInterceptor = &APIKeyInterceptor{}

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

func sign(data, key []byte) string {
	hasher := hmac.New(sha256.New, key)
	hasher.Write(data)
	sum := hasher.Sum(nil)

	return hex.EncodeToString(sum)
}
