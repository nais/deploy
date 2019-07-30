package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

// Request payload submitted when making a token request.
type Request struct {
	CircleCI CircleCI `json:"circleci,omitempty"`
}

type CircleCI struct {
	Repository string `json:"repository,omitempty"`
}

// Function that will issue tokens to remote services based on a Request payload.
type Issuer func(Request) error

type Handler struct {
	issuer Issuer
}

// Accept HTTP POST requests with a JSON payload that can be unmarshalled into a Request object.
// The Handler's issuer callback function will be called upon each request.
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

	request := Request{}

	err = json.Unmarshal(body, &request)
	if err != nil {
		response.WriteHeader(http.StatusBadRequest)
		response.Write([]byte(fmt.Sprintf("JSON error in request: %s\n", err)))
		return
	}

	err = h.issuer(request)
	if err != nil {
		response.WriteHeader(http.StatusInternalServerError)
		response.Write([]byte(fmt.Sprintf("Unable to issue token(s): %s\n", err)))
		return
	}

	response.WriteHeader(http.StatusCreated)
}

func New(issuer Issuer) *Handler {
	return &Handler{
		issuer: issuer,
	}
}
