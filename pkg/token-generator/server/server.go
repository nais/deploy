package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/token-generator/types"
	log "github.com/sirupsen/logrus"
)

// Function that will issue tokens to remote services based on a Request payload.
type Issuer func(types.Request) error

type Handler struct {
	issuer Issuer
}

const (
	CorrelationIDHeader = "X-Correlation-ID"
)

// Accept HTTP POST requests with a JSON payload that can be unmarshalled into a Request object.
// The Handler's issuer callback function will be called upon each request. This function must be thread-safe.
// Token issuing is synchronous, so when this function returns 201, clients can proceed with their task.
func (h *Handler) ServeHTTP(response http.ResponseWriter, httpRequest *http.Request) {
	if httpRequest.Method != http.MethodPost {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte("You must issue a POST request with a JSON payload to use this service.\n"))
		return
	}

	body, err := ioutil.ReadAll(httpRequest.Body)
	if err != nil {
		log.Errorf("reading request body: %s", err)
		response.WriteHeader(http.StatusInternalServerError)
		return
	}

	request := types.Request{
		ID: uuid.New().String(),
	}

	response.Header().Set(CorrelationIDHeader, request.ID)

	err = json.Unmarshal(body, &request)
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Sprintf("JSON error in request: %s\n", err)))
		return
	}

	err = request.Validate()
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Sprintf("%s\n", err)))
		return
	}

	err = h.issuer(request)
	if err != nil {
		response.WriteHeader(http.StatusServiceUnavailable)
		response.Write([]byte(fmt.Sprintf("%s\n", err)))
		return
	}

	response.WriteHeader(http.StatusCreated)
}

func New(issuer Issuer) *Handler {
	return &Handler{
		issuer: issuer,
	}
}
