package api_v1_teams

import (
	"encoding/json"
	"net/http"

	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	log "github.com/sirupsen/logrus"
)

type TeamsHandler struct {
	APIKeyStorage database.Database
}
type Team struct {
	Team    string `json:"team"`
	GroupId string `json:"groupId"`
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	groups, ok := r.Context().Value("groups").([]string)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	keys := []database.ApiKey{}
	for _, group := range groups {
		apiKeys, err := h.APIKeyStorage.ReadByGroupClaim(group)
		if err != nil {
			logger.Error(err)
		}
		if len(apiKeys) > 0 {
			for _, apiKey := range apiKeys {
				keys = append(keys, apiKey)
			}
		}
	}
	teams := []Team{}
	for _, v := range keys {
		t := Team{
			GroupId: v.GroupId,
			Team:    v.Team,
		}
		teams = append(teams, t)
	}

	response, err := json.Marshal(teams)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Unable to return any teams"))
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(response)
}
