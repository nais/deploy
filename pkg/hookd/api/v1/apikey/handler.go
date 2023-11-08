package api_v1_apikey

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"
	log "github.com/sirupsen/logrus"
)

type ApiKeyHandler interface {
	GetTeamApiKey(w http.ResponseWriter, r *http.Request)
	RotateTeamApiKey(w http.ResponseWriter, r *http.Request)
}

type DefaultApiKeyHandler struct {
	APIKeyStorage database.ApiKeyStore
}

// TeamApiKey returns the API key for a specific team
func (d *DefaultApiKeyHandler) GetTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	team := chi.URLParam(r, "team")

	keys, err := d.APIKeyStorage.ApiKeys(r.Context(), team)
	if err != nil {
		if database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		} else {
			w.WriteHeader(http.StatusBadGateway)
			logger.Errorf("%s: %s", "unable to communicate with team API key backend", err)
			return
		}
	}

	keys = keys.Valid()
	if len(keys) != 1 {
		w.WriteHeader(http.StatusBadGateway)
		logger.Errorf("expected exactly one valid key, got %d", len(keys))
		return
	}

	ret, err := json.Marshal(keys[0])
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to marshal api key: %s", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(ret)
}

// RotateTeamApiKey rotates the API key for a specific team
func (d *DefaultApiKeyHandler) RotateTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	logger.Tracef("Incoming internal api key rotate request")

	team := chi.URLParam(r, "team")
	key, err := api_v1.Keygen(api_v1.KeySize)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to generate API key: %s", err)
		return
	}
	if err := d.APIKeyStorage.RotateApiKey(r.Context(), team, key); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to rotate API key: %s", err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
