package server

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"time"

	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/middleware"

	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	// Maximum time, in seconds, that a request timestamp can differ from the current time.
	MaxTimeSkew = 30.0
)

type StatusHandler struct {
	APIKeyStorage persistence.ApiKeyStorage
	GithubClient  github.Client
}

type StatusRequest struct {
	DeploymentID int64  `json:"deploymentID"`
	Owner        string `json:"owner"`
	Repository   string `json:"repository"`
	Team         string `json:"team"`
	Timestamp    int64  `json:"timestamp"`
}

type StatusResponse struct {
	Message string  `json:"message,omitempty"`
	Status  *string `json:"status,omitempty"`
}

func (r *StatusResponse) render(w io.Writer) {
	json.NewEncoder(w).Encode(r)
}

func (r *StatusRequest) validate() error {

	if r.DeploymentID == 0 {
		return fmt.Errorf("no deployment id specified")
	}

	if len(r.Team) == 0 {
		return fmt.Errorf("no team specified")
	}

	if math.Abs(float64(r.Timestamp-time.Now().Unix())) > MaxTimeSkew {
		return fmt.Errorf("request is not within allowed timeframe")
	}

	return nil
}

func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var statusResponse StatusResponse

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	logger.Tracef("Incoming status request")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		statusResponse.Message = fmt.Sprintf("unable to read request body: %s", err)
		statusResponse.render(w)

		logger.Error(statusResponse.Message)
		return
	}

	encodedSignature := r.Header.Get(SignatureHeader)
	signature, err := hex.DecodeString(encodedSignature)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		statusResponse.Message = "HMAC digest must be hex encoded"
		statusResponse.render(w)
		logger.Errorf("unable to validate team: %s: %s", statusResponse.Message, err)
		return
	}

	logger.Tracef("Request has hex encoded data in signature header")

	statusRequest := &StatusRequest{}
	if err := json.Unmarshal(data, statusRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		statusResponse.Message = fmt.Sprintf("unable to unmarshal request body: %s", err)
		statusResponse.render(w)
		logger.Error(statusResponse.Message)
		return
	}

	logger.Tracef("Request has valid JSON")

	err = statusRequest.validate()

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		statusResponse.Message = fmt.Sprintf("invalid status request: %s", err)
		statusResponse.render(w)
		logger.Error(statusResponse.Message)
		return
	}

	logger.Tracef("Request body validated successfully")

	token, err := h.APIKeyStorage.Read(statusRequest.Team)

	if err != nil {
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			statusResponse.Message = FailedAuthenticationMsg
			statusResponse.render(w)
			logger.Errorf("%s: %s", FailedAuthenticationMsg, err)
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		statusResponse.Message = "something wrong happened when communicating with api key service"
		statusResponse.render(w)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	logger.Tracef("Team API key retrieved from storage")

	if !validateMAC(data, []byte(signature), token) {
		w.WriteHeader(http.StatusForbidden)
		statusResponse.Message = FailedAuthenticationMsg
		statusResponse.render(w)
		logger.Errorf("%s: HMAC signature error", FailedAuthenticationMsg)
		return
	}

	logger.Tracef("HMAC signature validated successfully")

	deploymentStatus, err := h.GithubClient.DeploymentStatus(
		r.Context(),
		statusRequest.Owner,
		statusRequest.Repository,
		statusRequest.DeploymentID,
	)

	if err != nil {
		if err == github.ErrNoDeploymentStatuses {
			w.WriteHeader(http.StatusNoContent)
			logger.Info("Deployment status requested but none available")
			return
		}
		w.WriteHeader(http.StatusBadGateway)
		statusResponse.Message = "unable to return deployment status: GitHub is unavailable"
		statusResponse.render(w)
		logger.Errorf("Unable to return deployment status: GitHub is unavailable: %s", err)
		return
	}

	w.WriteHeader(http.StatusOK)
	state := deploymentStatus.GetState()
	statusResponse.Status = &state
	statusResponse.Message = "deployment status retrieved successfully"
	statusResponse.render(w)

	logger.Info("Status request processed successfully")
}
