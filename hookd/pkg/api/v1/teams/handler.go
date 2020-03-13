package api_v1_teams

import (
	"fmt"
	"net/http"

	"github.com/dgrijalva/jwt-go"
)

type TeamsHandler struct {
}

func (h *TeamsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Converting groups from claims to group slice
	var groups []string
	claims := r.Context().Value("claims").(jwt.MapClaims)
	groupInterface := claims["groups"].([]interface{})
	groups = make([]string, len(groupInterface))
	for i, v := range groupInterface {
		groups[i] = v.(string)
	}

	fmt.Printf("groups: %s\n", groups)
	w.Write([]byte("Got teams "))
}
