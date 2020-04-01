package api_v1_status

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/middleware"

	types "github.com/navikt/deployment/common/pkg/deployment"
	log "github.com/sirupsen/logrus"
)

type StatusHandler struct {
	APIKeyStorage database.ApiKeyStore
	GithubClient  github.Client
}

type StatusRequest struct {
	DeploymentID int64            `json:"deploymentID"`
	Owner        string           `json:"owner"`
	Repository   string           `json:"repository"`
	Team         string           `json:"team"`
	Timestamp    api_v1.Timestamp `json:"timestamp"`
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
		return fmt.Errorf("no deployment ID specified")
	}

	if len(r.Owner) == 0 {
		return fmt.Errorf("no repository owner specified")
	}

	if len(r.Repository) == 0 {
		return fmt.Errorf("no repository specified")
	}

	if len(r.Team) == 0 {
		return fmt.Errorf("no team specified")
	}

	if err := r.Timestamp.Validate(); err != nil {
		return err
	}

	return nil
}

func (r *StatusRequest) LogFields() log.Fields {
	return log.Fields{
		types.LogFieldDeploymentID: r.DeploymentID,
		types.LogFieldTeam:         r.Team,
		types.LogFieldRepository:   fmt.Sprintf("%s/%s", r.Owner, r.Repository),
	}
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

	encodedSignature := r.Header.Get(api_v1.SignatureHeader)
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

	logger = logger.WithFields(statusRequest.LogFields())
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
	apiKeys, err := h.APIKeyStorage.ApiKeys(statusRequest.Team)

	if err != nil {
		if database.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			statusResponse.Message = api_v1.FailedAuthenticationMsg
			statusResponse.render(w)
			logger.Errorf("%s: %s", api_v1.FailedAuthenticationMsg, err)
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		statusResponse.Message = "something wrong happened when communicating with api key service"
		statusResponse.render(w)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	logger.Tracef("Team API keys retrieved from storage")

	err = api_v1.ValidateAnyMAC(data, signature, apiKeys.Valid().Keys())
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		statusResponse.Message = api_v1.FailedAuthenticationMsg
		statusResponse.render(w)
		logger.Error(err)
		return
	}

	logger.Tracef("HMAC signature validated successfully")

	logger.Tracef("Querying GitHub for deployment status")

	deploymentStatus, err := h.GithubClient.DeploymentStatus(
		r.Context(),
		statusRequest.Owner,
		statusRequest.Repository,
		statusRequest.DeploymentID,
	)

	if err != nil {
		if err == github.ErrDeploymentNotFound {
			w.WriteHeader(http.StatusBadRequest)
			logger.Infof("Deployment %d does not exist", statusRequest.DeploymentID)
			return
		} else if err == github.ErrNoDeploymentStatuses {
			w.WriteHeader(http.StatusNoContent)
			logger.Info("Deployment status requested, but none available yet")
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
