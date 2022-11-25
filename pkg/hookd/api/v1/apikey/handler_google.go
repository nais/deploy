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

type GoogleApiKeyHandler struct {
	APIKeyStorage database.ApiKeyStore
}

// This method returns all the keys the user is authorized to see
func (h *GoogleApiKeyHandler) GetApiKeys(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	logger.Tracef("Request API keys")

	groups, err := middleware.GetGroups(r.Context())
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
		} else {
			keys = append(keys, database.ApiKey{Team: group})
		}
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, keys)
}

// This method returns the deploy key for a specific team
func (h *GoogleApiKeyHandler) GetTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	groups, err := middleware.GetGroups(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	team := chi.URLParam(r, "team")
	hasAccess := false
	for _, group := range groups {
		if group == team {
			hasAccess = true
		}
	}

	if !hasAccess {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("not authorized to view this team's keys"))
		return
	}

	keys := make([]database.ApiKey, 0)
	apiKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), team)
	if err != nil {
		logger.Errorln(err)
		if database.IsErrNotFound(err) {
			keys = append(keys, database.ApiKey{Team: team})
		} else {
			w.WriteHeader(http.StatusBadGateway)
			logger.Errorf("unable to fetch team apikey from storage: %s", err)
			return
		}
	}
	for _, apiKey := range apiKeys {
		keys = append(keys, apiKey)
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, keys)
}

// This method rotates the api key for a specific team
func (h *GoogleApiKeyHandler) RotateTeamApiKey(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	groups, err := middleware.GetGroups(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	team := chi.URLParam(r, "team")
	hasAccess := false
	for _, group := range groups {
		if group == team {
			hasAccess = true
		}
	}
	if !hasAccess {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("not authorized to rotate this team's keys"))
		return
	}

	apiKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), team)
	if err != nil {
		if !database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Errorf("unable to fetch team apikey from storage: %s", err)
			return
		}
	}

	var keyToRotate *database.ApiKey
	for _, apiKey := range apiKeys {
		for _, group := range groups {
			if apiKey.Team == group {
				keyToRotate = &apiKey
			}
		}
	}
	if keyToRotate == nil {
		keyToRotate = &database.ApiKey{Team: team, GroupId: ""}
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
			if group == apiKey.Team {
				keys = append(keys, apiKey)
			}
		}
	}

	render.JSON(w, r, keys)
}
