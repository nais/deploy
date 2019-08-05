package types

import (
	"fmt"
	"net/http"
)

type APIKeyRequest struct {
	Team string `json:"team"`
}

type APIKeyResponse struct {
	Team   string `json:"team"`
	APIKey string `json:"apikey"`
}

func (r *APIKeyRequest) Bind(request *http.Request) error {
	if len(r.Team) == 0 {
		return fmt.Errorf("team name must be specified")
	}
	return nil
}
