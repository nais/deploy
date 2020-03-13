package api_v1_teams

import (
	"fmt"
	"net/http"
)

type TeamsHandler struct {
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Converting groups from claims to group slice

	fmt.Printf("groups: %s\n", r.Context().Value("groups"))
	w.Write([]byte("Got teams "))
}
