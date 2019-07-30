package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type Handler struct {
	Requests chan Request
}

type CircleCI struct {
	Repository string `json:"repository,omitempty"`
}

type Request struct {
	CircleCI CircleCI `json:"circleci,omitempty"`
}

func (h *Handler) ServeHTTP(response http.ResponseWriter, httpRequest *http.Request) {
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
		response.Write([]byte(fmt.Sprintf("JSON error in request: %s", err)))
		return
	}

	h.Requests <- request

	response.WriteHeader(http.StatusAccepted)
}

func New() *Handler {
	return &Handler{
		Requests: make(chan Request, 1024),
	}
}
