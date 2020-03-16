package api_v1_apikey

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/go-chi/chi"
	api_v1 "github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/middleware"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

type ApiKeyHandler struct {
	APIKeyStorage     persistence.ApiKeyStorage
}
type teamKey struct {
	Team    string `json:"team"`
	GroupId string `json:"groupId"`
	Key     string `json:"key"`
}

func (h *ApiKeyHandler) GetApiKeys(w http.ResponseWriter, r *http.Request) {
	// This method returns all the keys the user is authorized to see
	teamKeys := []teamKey{
		{
			Team:    "team1",
			GroupId: "6fbc76c4-7909-4e58-99fa-64d542567c8c",
			Key:     "xxxyyyyxxxyyy",
		},
		{
			Team:    "team2",
			GroupId: "a2a55070-3442-4a38-9a8d-c62fcf259158",
			Key:     "2xxxyyyyxxxyyy",
		},
		{
			Team:    "team3",
			GroupId: "6283f2bd-8bb5-4d13-ae38-974e1bcc1aad",
			Key:     "33333yyyxxxyyy",
		},
	}

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	tokens, err := h.APIKeyStorage.Read("team1")

	if err != nil {
		logger.Errorln(err)
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			//deploymentResponse.Message = api_v1.FailedAuthenticationMsg
			//deploymentResponse.render(w)
			logger.Errorf("%s: %s", api_v1.FailedAuthenticationMsg, err)
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		//deploymentResponse.Message = "something wrong happened when communicating with api key service"
		//deploymentResponse.render(w)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	for _, value := range tokens {
		fmt.Printf("TOKEN: %s\n", string(value))
	}

	groups := r.Context().Value("groups").([]string)
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
	}
	w.Write([]byte("Not authorized to fetch key for team"))
}

func (h *ApiKeyHandler) GetTeamApiKey(w http.ResponseWriter, r *http.Request) {
	teamkey := teamKey{
		Team:    "balls",
		GroupId: "6fbc76c4-7909-4e58-99fa-64d542567c8c",
		Key:     "xxxyyyyxxxyyy",
	}
	team := chi.URLParam(r, "team")
	// This method returns the deploy key for a specific team
	groups := r.Context().Value("groups").([]string)
	for _, v := range groups {
		if v == teamkey.GroupId && teamkey.Team == team {
			response, _ := json.Marshal(teamkey)
			w.Write(response)
			return
		}
	}
	w.Write([]byte("Not authorized to fetch key for team"))
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
