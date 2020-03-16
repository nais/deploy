package api_v1_apikey

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	api_v1 "github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	log "github.com/sirupsen/logrus"
)

type ApiKeyHandler struct {
	APIKeyStorage     database.Database
}

func (h *ApiKeyHandler) GetApiKeys(w http.ResponseWriter, r *http.Request) {
	// This method returns all the keys the user is authorized to see

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	//teamKey := database.TeamKey{}
	teamKeys, err := h.APIKeyStorage.Read("team1")

	if err != nil {
		logger.Errorln(err)
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			logger.Errorf("%s: %s", api_v1.FailedAuthenticationMsg, err)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	fmt.Printf("%#v", teamKeys)


/*	groups := r.Context().Value("groups").([]string)
	teams := []teamKey{}
	for _, group := range groups {
		for _, teamkey := range teamKeys {
			if group == teamkey.GroupId {
				teams = append(teams, teamkey)
			}
		}
	}
	if len(teams) > 0 {
		response, err := json.Marshal(teams)
		if err != nil {
			w.Write([]byte("Unable to fetch any team keys"))
			return
		}
		w.Write(response)
		return
	}*/
	w.Write([]byte("Not authorized to fetch key for team"))
}

func (h *ApiKeyHandler) GetTeamApiKey(w http.ResponseWriter, r *http.Request) {
	team := chi.URLParam(r, "team")
	// This method returns the deploy key for a specific team
	groups := r.Context().Value("groups").([]string)

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	teamKeys, err := h.APIKeyStorage.Read(team)

	if err != nil {
		logger.Errorln(err)
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			logger.Errorf("%s: %s", api_v1.FailedAuthenticationMsg, err)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}
	// Her fortsetter vi i morgen kommentar!
	for _, teamKey := range teamKeys {
		for _, group := range groups {
			if group == teamKey.GroupId {
				fmt.Printf("%#v\n", teamKey)
				json, err := json.Marshal(teamKey)
				if err != nil {
					w.WriteHeader(http.StatusInternalServerError)
					w.Write([]byte("Unable to marshal teamKey to json"))
					return
				}
				w.Write(json)
			}
		}
	}
	w.WriteHeader(http.StatusForbidden)
	w.Write([]byte("not authorized to view teamsKey"))
	return
}
func (h *ApiKeyHandler) RotateTeamApiKey(w http.ResponseWriter, r *http.Request) {
	// This method rotates the deploy key for a specific team
	key, err := api_v1.Keygen(32)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Unable to generate new random key"))
	}
	fmt.Printf("generated key: %s", key)
	groups := r.Context().Value("groups").([]string)
	groupString := "Group claims are: \n"
	for _, v := range groups {
		groupString += v + "\n"
	}
	response := []byte(groupString)
	w.WriteHeader(http.StatusNoContent)
	w.Write(response)
}
