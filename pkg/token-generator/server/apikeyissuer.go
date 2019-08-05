package server

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/navikt/deployment/pkg/token-generator/apikeys"
	"github.com/navikt/deployment/pkg/token-generator/httperr"
	"github.com/navikt/deployment/pkg/token-generator/types"
)

const (
	apiKeyEntropyBytes = 32
)

func NewAPIKeyIssuer(source apikeys.Source) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		request := types.APIKeyRequest{}

		err := render.Bind(r, &request)
		if err != nil {
			render.Render(w, r, httperr.ErrInvalidRequest(err))
			return
		}

		apikey, err := apikeys.New(apiKeyEntropyBytes)
		if err != nil {
			render.Render(w, r, httperr.ErrUnavailable(err))
			return
		}

		responseKey := types.APIKeyResponse{
			Team:   request.Team,
			APIKey: apikey,
		}

		err = source.Write(responseKey.Team, responseKey.APIKey)
		if err != nil {
			render.Render(w, r, httperr.ErrUnavailable(err))
			return
		}

		w.WriteHeader(http.StatusCreated)
		render.JSON(w, r, responseKey)
	}
}
