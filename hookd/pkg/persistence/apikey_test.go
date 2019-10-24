package persistence_test

import (
	"fmt"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

const (
	Existing    = "existing"
	Unavailable = "unavailable"
	Nonexistent = "nonexistent"
	ApiKey      = "topsecret"
)

func TestVaultApiKeyStorage(t *testing.T) {
	defaults := config.DefaultConfig().Vault
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch teamname(r.URL) {
		case Existing:
			_, _ = io.WriteString(w,
				fmt.Sprintf(`{
                    "data": {
                      "%s": "%s"
                    }
                 }`, defaults.KeyName, ApiKey))
		case Nonexistent:
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte("not found"))
		case Unavailable:
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal server error"))
		default:
			panic("we should not be here")
		}
	}))

	defer server.Close()

	vault := persistence.VaultApiKeyStorage{
		HttpClient: server.Client(),
		Address:    server.URL,
		Path:       defaults.Path,
		KeyName:    defaults.KeyName,
		Token:      defaults.Token,
	}

	t.Run("finds api keys in vault", func(t *testing.T) {
		apiKey, err := vault.Read(Existing)
		assert.NoError(t, err)
		assert.Equal(t, []byte(ApiKey), apiKey)
	})

	t.Run("fails when team doesnt exist", func(t *testing.T) {
		_, err := vault.Read(Nonexistent)
		assert.Error(t, err)
		assert.True(t, vault.IsErrNotFound(err))
	})

	t.Run("returns an error when communication with vault fails somehow", func(t *testing.T) {
		_, err := vault.Read(Unavailable)
		assert.Error(t, err)
	})
}

func teamname(u *url.URL) string {
	fragments := strings.Split(u.Path, "/")
	return fragments[len(fragments)-1]
}
