package api_v1_teams

import (
	"net/http"
)

type TeamsHandler struct {
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	groups := r.Context().Value("groups").([]string)
	groupString := "Group claims are: \n"
	for _, v := range groups {
		groupString += v + "\n"
	}
	response := []byte(groupString)
	w.Write(response)
}
