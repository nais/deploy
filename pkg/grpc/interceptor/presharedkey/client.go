package presharedkey_interceptor

import (
	"context"
)

type ClientInterceptor struct {
	RequireTLS bool
	Key        string
}

func (t *ClientInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": t.Key}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}
