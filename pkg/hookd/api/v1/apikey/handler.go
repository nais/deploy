package api_v1_apikey

import "net/http"

type ApiKeyHandler interface {
	GetTeamApiKey(w http.ResponseWriter, r *http.Request)
	GetApiKeys(w http.ResponseWriter, r *http.Request)
	RotateTeamApiKey(w http.ResponseWriter, r *http.Request)
}
