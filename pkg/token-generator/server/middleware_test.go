package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/navikt/deployment/pkg/token-generator/server"
	"github.com/stretchr/testify/assert"
)

func TestApiKeyMiddlewareHandler(t *testing.T) {
	succeed := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("test-success", "true")
	})

	fail := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Fail(t, "request is passed through without a valid API key")
	})

	t.Run("missing API key results in a blocked request", func(t *testing.T) {
		source := apikeys.NewMemoryStore()
		middleware := server.ApiKeyMiddlewareHandler(source)
		handler := middleware(fail)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/create", nil)

		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("invalid API keys are blocked", func(t *testing.T) {
		source := apikeys.NewMemoryStore()
		middleware := server.ApiKeyMiddlewareHandler(source)
		handler := middleware(fail)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/create", nil)

		// Create API key pair and use it in basic auth
		assert.NoError(t, source.Write("foo", "bar"))
		r.SetBasicAuth("invalid", "keypair")

		handler.ServeHTTP(w, r)

		assert.Equal(t, http.StatusForbidden, w.Code)
	})

	t.Run("valid API keys pass through middleware", func(t *testing.T) {
		source := apikeys.NewMemoryStore()
		middleware := server.ApiKeyMiddlewareHandler(source)
		handler := middleware(succeed)

		w := httptest.NewRecorder()
		r := httptest.NewRequest("POST", "/create", nil)

		// Create API key pair and use it in basic auth
		assert.NoError(t, source.Write("foo", "bar"))
		r.SetBasicAuth("foo", "bar")

		handler.ServeHTTP(w, r)

		assert.Equal(t, "true", w.Header().Get("test-success"))
	})
}

func TestJWTMiddlewareHandler(t *testing.T) {
}
