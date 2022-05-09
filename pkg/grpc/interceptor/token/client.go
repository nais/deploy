package token_interceptor

import (
	"context"
)

type ClientInterceptor struct {
	RequireTLS bool
	Token      string
}

func (t *ClientInterceptor) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{"authorization": t.Token}, nil
}

func (t *ClientInterceptor) RequireTransportSecurity() bool {
	return t.RequireTLS
}
