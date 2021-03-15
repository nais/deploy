package api_v1_apikey

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"
	log "github.com/sirupsen/logrus"
)

type ApiKeyHandler struct {
	APIKeyStorage database.ApiKeyStore
}

// This method returns all the keys the user is authorized to see
func (h *ApiKeyHandler) GetApiKeys(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	logger.Tracef("Request API keys")

	groups, err := api_v1.GroupClaims(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	keys := make([]database.ApiKey, 0)

	for _, group := range groups {
		groupKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), group)
		if err != nil {
			logger.Tracef("Group claim: %s: %s", group, err)
		}
		if len(groupKeys) > 0 {
			for _, groupKey := range groupKeys {
				keys = append(keys, groupKey)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, keys)
}

// This method returns the deploy key for a specific team
func (h *ApiKeyHandler) GetTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	groups, err := api_v1.GroupClaims(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	team := chi.URLParam(r, "team")
	apiKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), team)

	if err != nil {
		logger.Errorln(err)
		if database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			logger.Errorf("%s: %s", api_v1.FailedAuthenticationMsg, err)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	keys := make([]database.ApiKey, 0)
	for _, apiKey := range apiKeys {
		for _, group := range groups {
			if group == apiKey.GroupId {
				keys = append(keys, apiKey)
			}
		}
	}

	if len(keys) == 0 {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("not authorized to view this team's keys"))
		return
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, keys)
}

// This method rotates the api key for a specific team
func (h *ApiKeyHandler) RotateTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	groups, err := api_v1.GroupClaims(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	team := chi.URLParam(r, "team")
	apiKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), team)

	if err != nil {
		if database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
			logger.Errorf("team does not exist: %s", err)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	var keyToRotate database.ApiKey
	for _, apiKey := range apiKeys {
		for _, group := range groups {
			if apiKey.GroupId == group && apiKey.Team == team {
				keyToRotate = apiKey
			}
		}
	}

	if len(keyToRotate.GroupId) == 0 {
		w.WriteHeader(http.StatusForbidden)
		return
	}

	newKey, err := api_v1.Keygen(32)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to generate new random api key: %s", err)
		return
	}
	logger.Infof("generated new api key for %s (%s)", keyToRotate.Team, keyToRotate.GroupId)
	err = h.APIKeyStorage.RotateApiKey(r.Context(), keyToRotate.Team, keyToRotate.GroupId, newKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Errorf("unable to persist api key: %s", err)
		return
	}

	w.WriteHeader(http.StatusCreated)

	apiKeys, err = h.APIKeyStorage.ApiKeys(r.Context(), team)
	if err != nil {
		return // api keys were created, but we cannot return the full list
	}

	keys := make([]database.ApiKey, 0)
	for _, apiKey := range apiKeys {
		for _, group := range groups {
			if group == apiKey.GroupId {
				keys = append(keys, apiKey)
			}
		}
	}

	render.JSON(w, r, keys)
}
