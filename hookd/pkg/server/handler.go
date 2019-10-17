package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"io/ioutil"
	"net/http"

	gh "github.com/google/go-github/v27/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	SignatureHeader         = "X-NAIS-Signature"
	FailedAuthenticationMsg = "failed authentication"
)

type DeploymentHandler struct {
	log               *log.Entry
	SecretToken       string
	APIKeyStorage     persistence.ApiKeyStorage
	DeploymentStatus  chan types.DeploymentStatus
	DeploymentRequest chan types.DeploymentRequest
	DeploymentCreator func(DeploymentRequest) (*gh.Deployment, error)
}

type DeploymentRequest struct {
	Resources  json.RawMessage `json:"resources,omitempty"`
	Team       string          `json:"team,omitempty"`
	Cluster    string          `json:"cluster,omitempty"`
	Owner      string          `json:"owner,omitempty"`
	Repository string          `json:"repository,omitempty"`
	Ref        string          `json:"ref,omitempty"`
}

type DeploymentResponse struct {
	GithubDeployment *gh.Deployment `json:"githubDeployment,omitempty"`
	CorrelationID    string         `json:"correlationID,omitempty"`
	Message          string         `json:"message,omitempty"`
}

func (r *DeploymentResponse) render(w io.Writer) {
	json.NewEncoder(w).Encode(r)
}

func (r *DeploymentRequest) validate() error {

	if len(r.Owner) == 0 {
		return fmt.Errorf("no repository owner specified")
	}

	if len(r.Repository) == 0 {
		return fmt.Errorf("no repository specified")
	}

	if len(r.Cluster) == 0 {
		return fmt.Errorf("no cluster specified")
	}

	if len(r.Team) == 0 {
		return fmt.Errorf("no team specified")
	}

	if len(r.Resources) == 0 {
		return fmt.Errorf("no resources specified")
	}

	return nil
}

func (h *DeploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var deploymentResponse DeploymentResponse
	correlationID, err := uuid.NewRandom()

	h.log = log.WithFields(log.Fields{
		types.LogFieldDeliveryID: correlationID,
	})

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		deploymentResponse.Message = fmt.Sprintf("unable to create correlation id: %s", err)
		deploymentResponse.render(w)
		return
	}

	deploymentResponse.CorrelationID = correlationID.String()

	data, err := ioutil.ReadAll(r.Body)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		deploymentResponse.Message = fmt.Sprintf("unable to read request body: %s", err)
		deploymentResponse.render(w)
		return
	}

	h.log.Infof("Received %s request on %s", r.Method, r.RequestURI)

	deploymentRequest := &DeploymentRequest{}
	if err := json.Unmarshal(data, deploymentRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = fmt.Sprintf("unable to unmarshal request body: %s", err)
		deploymentResponse.render(w)
		return
	}

	if err := deploymentRequest.validate(); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = fmt.Sprintf("invalid deployment request: %s", err)
		deploymentResponse.render(w)
		return
	}

	token, err := h.APIKeyStorage.Read(deploymentRequest.Team)

	if err != nil {
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			deploymentResponse.Message = FailedAuthenticationMsg
			deploymentResponse.render(w)
			return
		}

		w.WriteHeader(http.StatusServiceUnavailable)
		deploymentResponse.Message = "unable to fetch team apikey from storage"
		deploymentResponse.render(w)
		h.log.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	signature := r.Header.Get(SignatureHeader)

	if !validateMAC(data, []byte(signature), token) {
		w.WriteHeader(http.StatusForbidden)
		deploymentResponse.Message = FailedAuthenticationMsg
		deploymentResponse.render(w)
		return
	}

	deployment, err := h.DeploymentCreator(*deploymentRequest)
	deploymentResponse.GithubDeployment = deployment

	if err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		deploymentResponse.Message = "unable to create GitHub deployment"
		deploymentResponse.render(w)
		h.log.Errorf("unable to create GitHub deployment: %s", err)
		return
	}

	deployMsg, err := DeploymentRequestMessage(deploymentRequest, deployment, correlationID.String())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = "unable to create deployment message"
		deploymentResponse.render(w)
		h.log.Errorf("unable to create deployment message: %s", err)
		return
	}

	h.DeploymentRequest <- *deployMsg

	w.WriteHeader(http.StatusCreated)
	deploymentResponse.render(w)
	h.log.Infof("created deployment message to cluster %s for repo %s/%s", deploymentRequest.Cluster, deploymentRequest.Owner, deploymentRequest.Repository)
}

// validateMAC reports whether messageMAC is a valid HMAC tag for message.
func validateMAC(message, messageMAC, key []byte) bool {
	expectedMAC := genMAC(message, key)
	return hmac.Equal(messageMAC, expectedMAC)
}

// genMAC generates the HMAC signature for a message provided the secret key using SHA256
func genMAC(message, key []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(message)
	return mac.Sum(nil)
}
