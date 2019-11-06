package server

import (
	"crypto/hmac"
	"crypto/sha256"
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

	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	SignatureHeader         = "X-NAIS-Signature"
	FailedAuthenticationMsg = "failed authentication"
	MaxTimeSkew             = 30.0
)

type StatusHandler struct {
	APIKeyStorage persistence.ApiKeyStorage
	GithubClient  github.Client
}

type StatusRequest struct {
	DeploymentId string `json:"deploymentId"`
	Team         string `json:"team"`
	Timestamp    int64  `json:"timestamp"`
}

type StatusResponse struct {
	Message string                      `json:"message,omitempty"`
	Status  types.GithubDeploymentState `json:"status,omitempty"`
}

func (r *StatusResponse) render(w io.Writer) {
	json.NewEncoder(w).Encode(r)
}

func (r *StatusRequest) validate() error {

	if len(r.DeploymentId) == 0 {
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

//func (r *DeploymentRequest) GithubDeploymentRequest() gh.DeploymentRequest {
//	return gh.DeploymentRequest{
//		Environment: gh.String(r.Cluster),
//		Ref:         gh.String(r.Ref),
//		Task:        gh.String(DirectDeployGithubTask),
//	}
//}
//

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
	//
	//err = h.GithubClient.TeamAllowed(r.Context(), statusRequest.Owner, statusRequest.Repository, statusRequest.Team)
	//switch err {
	//case nil:
	//	logger.Tracef("Team access to repository on GitHub validated successfully")
	//case github.ErrTeamNotExist, github.ErrTeamNoAccess:
	//	statusResponse.Message = err.Error()
	//	w.WriteHeader(http.StatusForbidden)
	//	statusResponse.render(w)
	//	logger.Error(err)
	//	return
	//default:
	//	statusResponse.Message = "unable to communicate with GitHub"
	//	w.WriteHeader(http.StatusBadGateway)
	//	statusResponse.render(w)
	//	logger.Errorf("%s: %s", statusResponse.Message, err)
	//	return
	//}
	//
	//githubRequest := statusRequest.GithubDeploymentRequest()
	//githubDeployment, err = h.GithubClient.CreateDeployment(r.Context(), statusRequest.Owner, statusRequest.Repository, &githubRequest)
	//statusResponse.GithubDeployment = githubDeployment
	//
	//if err != nil {
	//	w.WriteHeader(http.StatusBadGateway)
	//	statusResponse.Message = "unable to create GitHub deployment"
	//	statusResponse.render(w)
	//	logger.Errorf("unable to create GitHub deployment: %s", err)
	//	return
	//}
	//
	//logger = logger.WithField(types.LogFieldDeploymentID, githubDeployment.GetID())
	//logger.Info("GitHub deployment created successfully")
	//
	//deployMsg, err := DeploymentRequestMessage(statusRequest, githubDeployment, statusResponse.CorrelationID)
	//if err != nil {
	//	w.WriteHeader(http.StatusBadRequest)
	//	statusResponse.Message = "unable to create deployment message"
	//	statusResponse.render(w)
	//	logger.Errorf("unable to create deployment message: %s", err)
	//	return
	//}
	//
	//h.DeploymentRequest <- *deployMsg
	//
	//w.WriteHeader(http.StatusCreated)
	//statusResponse.Message = "deployment request accepted and dispatched"
	//statusResponse.render(w)

	//logger.Info("Deployment request processed successfully")
}

// validateMAC reports whether messageMAC is a valid HMAC tag for message.
func validateMAC(message, messageMAC, key []byte) bool {
	expectedMAC := GenMAC(message, key)
	return hmac.Equal(messageMAC, expectedMAC)
}

// GenMAC generates the HMAC signature for a message provided the secret key using SHA256
func GenMAC(message, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}
