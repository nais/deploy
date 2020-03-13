package api_v1_teams

import (
	"net/http"
)

type TeamsHandler struct {
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Got teams"))
}
