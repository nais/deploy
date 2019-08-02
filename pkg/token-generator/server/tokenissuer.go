package server

import (
	"net/http"

	"github.com/go-chi/render"
	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/token-generator/httperr"
	"github.com/navikt/deployment/pkg/token-generator/types"
)

// Function that will issue tokens to remote services based on a Request payload.
type Issuer func(types.Request) error

type TokenIssuerHandler struct {
	issuer Issuer
}

const (
	CorrelationIDHeader = "X-Correlation-ID"
)

// Accept HTTP POST requests with a JSON payload that can be unmarshalled into a Request object.
// The Handler's issuer callback function will be called upon each request. This function must be thread-safe.
// Token issuing is synchronous, so when this function returns 201, clients can proceed with their task.
func (h *TokenIssuerHandler) ServeHTTP(response http.ResponseWriter, httpRequest *http.Request) {
	request := types.Request{}

	err := render.Bind(httpRequest, &request)
	if err != nil {
		render.Render(response, httpRequest, httperr.ErrInvalidRequest(err))
		return
	}

	request.ID = uuid.New().String()
	request.Context = httpRequest.Context()

	response.Header().Set(CorrelationIDHeader, request.ID)

	err = h.issuer(request)
	if err != nil {
		render.Render(response, httpRequest, httperr.ErrUnavailable(err))
		return
	}

	response.WriteHeader(http.StatusCreated)
}

func New(issuer Issuer) *TokenIssuerHandler {
	return &TokenIssuerHandler{
		issuer: issuer,
	}
}
