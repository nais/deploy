package api_v1_apikey_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi"

	"github.com/nais/deploy/pkg/hookd/api"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"
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
			Team: "team1",
			Key:  key1,
		}}, nil
	case "team2":
		return database.ApiKeys{{
			Team: "team2",
			Key:  key2,
		}}, nil
	case "team4":
		return database.ApiKeys{{
			Team: "team4",
			Key:  key4,
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
	})

	t.Run("get teams", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("GET", "/api/v1/teams", nil)
		request = request.WithContext(middleware.WithGroups(request.Context(), []string{"team1", "team2", "team6"}))
		handler.ServeHTTP(recorder, request)

		var apikeys database.ApiKeys
		err := json.NewDecoder(recorder.Body).Decode(&apikeys)
		assert.NoError(t, err)

		assert.Equal(t, http.StatusOK, recorder.Code)
		assert.Len(t, apikeys, 2)
		assert.Equal(t, "team1", apikeys[0].Team)
		assert.Equal(t, "team2", apikeys[1].Team)
		assert.Equal(t, api_v1.Key(nil), apikeys[0].Key)
		assert.Equal(t, api_v1.Key(nil), apikeys[1].Key)
	})

	t.Run("get apikey for team", func(t *testing.T) {
		t.Run("team member", func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/v1/apikey/team2", nil)
			request = request.WithContext(middleware.WithGroups(request.Context(), []string{"team1", "team2", "team6"}))

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			body := string(recorder.Body.Bytes())

			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, `[{"team":"team2","key":"313233343536","expires":"0001-01-01T00:00:00Z","created":"0001-01-01T00:00:00Z"}]`+"\n", body)
		})
		t.Run("not team member", func(t *testing.T) {
			request := httptest.NewRequest("GET", "/api/v1/apikey/team5", nil)
			request = request.WithContext(middleware.WithGroups(request.Context(), []string{"team1", "team2", "team6"}))

			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			body := string(recorder.Body.Bytes())

			assert.Equal(t, http.StatusForbidden, recorder.Code)
			assert.Equal(t, `not authorized to view this team's keys`, body)
		})
	})
	// t.Run("rotate team", func(t *testing.T) {
	// 	request := httptest.NewRequest("POST", "/api/v1/apikey/team1", nil)
	// })
}
