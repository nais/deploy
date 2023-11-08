package api_v1_apikey_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"time"

	"github.com/go-chi/chi"

	"github.com/nais/deploy/pkg/hookd/api"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/stretchr/testify/assert"
)

type apiKeyStorage struct{}

var (
	key1 = []byte("abcdef")
	key2 = []byte("123456")
	key4 = api_v1.Key{0x00} // not used
)

func tokenValidatorMiddleware(next http.Handler) http.Handler {
	return next
}

func (a *apiKeyStorage) ApiKeys(ctx context.Context, id string) (database.ApiKeys, error) {
	switch id {
	case "team1":
		return database.ApiKeys{{
			Team:    "team1",
			Key:     key1,
			Expires: time.Now().Add(1 * time.Minute),
		}}, nil
	case "team2":
		return database.ApiKeys{{
			Team:    "team2",
			Key:     key2,
			Expires: time.Now().Add(1 * time.Minute),
		}}, nil
	case "team4":
		return database.ApiKeys{{
			Team:    "team4",
			Key:     key4,
			Expires: time.Now().Add(1 * time.Minute),
		}}, nil
	default:
		return nil, fmt.Errorf("err")
	}
}

func (a *apiKeyStorage) RotateApiKey(ctx context.Context, team string, key api_v1.Key) error {
	switch team {
	case "team1":
		return nil
	}
	return fmt.Errorf("err")
}

func TestApiKeyHandler(t *testing.T) {
	apiKeyStore := apiKeyStorage{}
	handler := api.New(api.Config{
		ApiKeyStore:          &apiKeyStore,
		MetricsPath:          "/metrics",
		ValidatorMiddlewares: chi.Middlewares{tokenValidatorMiddleware},
		PSKValidator: func(h http.Handler) http.Handler {
			return h
		},
	})

	t.Run("get apikey for team", func(t *testing.T) {
		request := httptest.NewRequest("GET", "/internal/api/v1/console/apikey/team2", nil)

		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, request)

		body := recorder.Body.String()

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Regexp(t, regexp.MustCompile(`{"team":"team2","key":"313233343536","expires":"\d{4}-\d{2}-[^"]+","created":"0001-01-01T00:00:00Z"}`), body)
	})

	// t.Run("get apikey for team", func(t *testing.T) {
	// 	request := httptest.NewRequest("GET", "/internal/api/v1/apikey/team2", nil)
	// 	request = request.WithContext(middleware.WithGroups(request.Context(), []string{"team1", "team2", "team6"}))
	//
	// 	recorder := httptest.NewRecorder()
	// 	handler.ServeHTTP(recorder, request)
	//
	// 	body := recorder.Body.String()
	//
	// 	assert.Equal(t, http.StatusOK, recorder.Code)
	// 	assert.Equal(t, `[{"team":"team2","key":"313233343536","expires":"0001-01-01T00:00:00Z","created":"0001-01-01T00:00:00Z"}]`+"\n", body)
	// })
}
