package auth_interceptor

import (
	"context"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestBearerHeaderIsSetWhenOidcTokenPresent(t *testing.T) {
	i := ClientInterceptor{
		RequireTLS: false,
		APIKey:     nil,
		OIDCToken:  "blabla",
		Team:       "team test",
	}
	requestMeta, _ := i.GetRequestMetadata(context.Background(), "")
	assert.Equal(t, "Bearer blabla", requestMeta["authorization"])
}

func TestCustomAuthHeaderIsSetWhenApiKeyPresent(t *testing.T) {
	i := ClientInterceptor{
		RequireTLS: false,
		APIKey:     []byte("whatever"),
		OIDCToken:  "",
		Team:       "team test",
	}
	requestMeta, _ := i.GetRequestMetadata(context.Background(), "")
	assert.False(t, strings.HasPrefix(requestMeta["authorization"], "Bearer"))
}
