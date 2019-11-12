package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/logproxy"
	"github.com/navikt/deployment/hookd/pkg/middleware"

	gh "github.com/google/go-github/v27/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	SignatureHeader         = "X-NAIS-Signature"
	FailedAuthenticationMsg = "failed authentication"
	DirectDeployGithubTask  = "NAIS_DIRECT_DEPLOY"
)

type ClusterList []string

type DeploymentHandler struct {
	APIKeyStorage     persistence.ApiKeyStorage
	GithubClient      github.Client
	DeploymentStatus  chan types.DeploymentStatus
	DeploymentRequest chan types.DeploymentRequest
	BaseURL           string
	Clusters          ClusterList
}

type DeploymentRequest struct {
	Resources  json.RawMessage `json:"resources,omitempty"`
	Team       string          `json:"team,omitempty"`
	Cluster    string          `json:"cluster,omitempty"`
	Owner      string          `json:"owner,omitempty"`
	Repository string          `json:"repository,omitempty"`
	Ref        string          `json:"ref,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}

type DeploymentResponse struct {
	Message          string         `json:"message,omitempty"`
	CorrelationID    string         `json:"correlationID,omitempty"`
	LogURL           string         `json:"logURL,omitempty"`
	GithubDeployment *gh.Deployment `json:"githubDeployment,omitempty"`
}

func (c ClusterList) Contains(cluster string) error {
	for _, cl := range c {
		if cl == cluster {
			return nil
		}
	}
	return fmt.Errorf("cluster '%s' is not a valid choice", cluster)
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

	if len(r.Ref) == 0 {
		return fmt.Errorf("no commit ref specified")
	}

	list := make([]interface{}, 0)
	err := json.Unmarshal(r.Resources, &list)
	if err != nil {
		return fmt.Errorf("resources field must be a list")
	} else if len(list) == 0 {
		return fmt.Errorf("resources must contain at least one Kubernetes resource")
	}

	return nil
}

func (r *DeploymentRequest) GithubDeploymentRequest() gh.DeploymentRequest {
	return gh.DeploymentRequest{
		Environment: gh.String(r.Cluster),
		Ref:         gh.String(r.Ref),
		Task:        gh.String(DirectDeployGithubTask),
	}
}

func (h *DeploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var err error
	var deploymentResponse DeploymentResponse
	var githubDeployment *gh.Deployment

	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	requestID, err := uuid.NewRandom()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		deploymentResponse.Message = fmt.Sprintf("unable to generate request id")
		deploymentResponse.render(w)
		logger.Errorf("%s: %s", deploymentResponse.Message, err)
		return
	}

	logger = logger.WithField(types.LogFieldDeliveryID, requestID.String())
	deploymentResponse.CorrelationID = requestID.String()
	deploymentResponse.LogURL = logproxy.MakeURL(h.BaseURL, requestID.String(), time.Now())

	logger.Tracef("Incoming request")

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		deploymentResponse.Message = fmt.Sprintf("unable to read request body: %s", err)
		deploymentResponse.render(w)
		logger.Error(deploymentResponse.Message)
		return
	}

	encodedSignature := r.Header.Get(SignatureHeader)
	signature, err := hex.DecodeString(encodedSignature)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = "HMAC digest must be hex encoded"
		deploymentResponse.render(w)
		logger.Errorf("unable to validate team: %s: %s", deploymentResponse.Message, err)
		return
	}

	logger.Tracef("Request has hex encoded data in signature header")

	deploymentRequest := &DeploymentRequest{}
	if err := json.Unmarshal(data, deploymentRequest); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = fmt.Sprintf("unable to unmarshal request body: %s", err)
		deploymentResponse.render(w)
		logger.Error(deploymentResponse.Message)
		return
	}

	logger = logger.WithFields(log.Fields{
		types.LogFieldTeam:       deploymentRequest.Team,
		types.LogFieldCluster:    deploymentRequest.Cluster,
		types.LogFieldRepository: fmt.Sprintf("%s/%s", deploymentRequest.Owner, deploymentRequest.Repository),
	})

	logger.Tracef("Request has valid JSON")

	err = deploymentRequest.validate()
	if err == nil {
		err = h.Clusters.Contains(deploymentRequest.Cluster)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = fmt.Sprintf("invalid deployment request: %s", err)
		deploymentResponse.render(w)
		logger.Error(deploymentResponse.Message)
		return
	}

	logger.Tracef("Request body validated successfully")

	token, err := h.APIKeyStorage.Read(deploymentRequest.Team)

	if err != nil {
		if h.APIKeyStorage.IsErrNotFound(err) {
			w.WriteHeader(http.StatusForbidden)
			deploymentResponse.Message = FailedAuthenticationMsg
			deploymentResponse.render(w)
			logger.Errorf("%s: %s", FailedAuthenticationMsg, err)
			return
		}

		w.WriteHeader(http.StatusBadGateway)
		deploymentResponse.Message = "something wrong happened when communicating with api key service"
		deploymentResponse.render(w)
		logger.Errorf("unable to fetch team apikey from storage: %s", err)
		return
	}

	logger.Tracef("Team API key retrieved from storage")

	if !validateMAC(data, []byte(signature), token) {
		w.WriteHeader(http.StatusForbidden)
		deploymentResponse.Message = FailedAuthenticationMsg
		deploymentResponse.render(w)
		logger.Errorf("%s: HMAC signature error", FailedAuthenticationMsg)
		return
	}

	logger.Tracef("HMAC signature validated successfully")

	err = h.GithubClient.TeamAllowed(r.Context(), deploymentRequest.Owner, deploymentRequest.Repository, deploymentRequest.Team)
	switch err {
	case nil:
		logger.Tracef("Team access to repository on GitHub validated successfully")
	case github.ErrTeamNotExist, github.ErrTeamNoAccess:
		deploymentResponse.Message = err.Error()
		w.WriteHeader(http.StatusForbidden)
		deploymentResponse.render(w)
		logger.Error(err)
		return
	default:
		deploymentResponse.Message = "unable to communicate with GitHub"
		w.WriteHeader(http.StatusBadGateway)
		deploymentResponse.render(w)
		logger.Errorf("%s: %s", deploymentResponse.Message, err)
		return
	}

	githubRequest := deploymentRequest.GithubDeploymentRequest()
	githubDeployment, err = h.GithubClient.CreateDeployment(r.Context(), deploymentRequest.Owner, deploymentRequest.Repository, &githubRequest)
	deploymentResponse.GithubDeployment = githubDeployment

	if err != nil {
		w.WriteHeader(http.StatusBadGateway)
		deploymentResponse.Message = "unable to create GitHub deployment"
		deploymentResponse.render(w)
		logger.Errorf("unable to create GitHub deployment: %s", err)
		return
	}

	logger = logger.WithField(types.LogFieldDeploymentID, githubDeployment.GetID())
	logger.Info("GitHub deployment created successfully")

	deployMsg, err := DeploymentRequestMessage(deploymentRequest, githubDeployment, deploymentResponse.CorrelationID)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		deploymentResponse.Message = "unable to create deployment message"
		deploymentResponse.render(w)
		logger.Errorf("unable to create deployment message: %s", err)
		return
	}

	h.DeploymentRequest <- *deployMsg

	w.WriteHeader(http.StatusCreated)
	deploymentResponse.Message = "deployment request accepted and dispatched"
	deploymentResponse.render(w)

	logger.Info("Deployment request processed successfully")
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
