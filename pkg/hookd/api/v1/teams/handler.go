package api_v1_teams

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"
	log "github.com/sirupsen/logrus"
)

type TeamsHandler struct {
	APIKeyStorage database.ApiKeyStore
}
type Team struct {
	Team    string `json:"team"`
	GroupId string `json:"groupId"`
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	groups, err := middleware.GetGroups(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	keys := make([]database.ApiKey, 0)
	for _, group := range groups {
		apiKeys, err := h.APIKeyStorage.ApiKeys(r.Context(), group)
		if err != nil {
			logger.Error(err)
		}
		if len(apiKeys) > 0 {
			for _, apiKey := range apiKeys {
				keys = append(keys, apiKey)
			}
		}
	}
	teams := make([]Team, 0)
	for _, v := range keys {
		t := Team{
			GroupId: v.GroupId,
			Team:    v.Team,
		}
		teams = append(teams, t)
	}

	w.WriteHeader(http.StatusOK)
	render.JSON(w, r, teams)
}
