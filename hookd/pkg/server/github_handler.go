package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	gh "github.com/google/go-github/v27/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	webhookTypeDeployment = "deployment"
)

type GithubDeploymentHandler struct {
	log                   *log.Entry
	SecretToken           string
	TeamRepositoryStorage persistence.TeamRepositoryStorage
	DeploymentStatus      chan types.DeploymentStatus
	DeploymentRequest     chan types.DeploymentRequest
	Clusters              ClusterList
}

func (h *GithubDeploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code, err := h.handler(r)

	metrics.WebhookRequest(code)

	w.WriteHeader(code)

	if err == nil {
		h.log.Infof("Request finished successfully with status code %d", code)
		return
	}

	if code < 400 {
		h.log.Infof("Request finished successfully with status code %d: %s", code, err)
		return
	}

	h.log.Errorf("Request failed with status code %d: %s", code, err)
	_, err = w.Write([]byte(err.Error()))
	if err == nil {
		return
	}

	h.log.Errorf("Additionally, while responding to HTTP request: %s", err)
}

func (h *GithubDeploymentHandler) validateTeamAccess(req *types.DeploymentRequest) error {
	fullName := req.GetDeployment().GetRepository().FullName()
	allowedTeams, err := h.TeamRepositoryStorage.Read(fullName)
	if err != nil {
		if h.TeamRepositoryStorage.IsErrNotFound(err) {
			return fmt.Errorf("repository '%s' is not registered", fullName)
		}
		return fmt.Errorf("unable to check if repository has team access: %s", err)
	}

	team := req.GetPayloadSpec().GetTeam()
	if len(team) == 0 {
		return fmt.Errorf("no team was specified in deployment payload")
	}

	for _, allowedTeam := range allowedTeams {
		if allowedTeam == team {
			return nil
		}
	}

	return fmt.Errorf("the repository '%s' does not have access to deploy as team '%s'", fullName, team)
}

func (h *GithubDeploymentHandler) handler(r *http.Request) (int, error) {
	var err error

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	eventType := r.Header.Get("X-GitHub-Event")
	sig := r.Header.Get("X-Hub-Signature")

	h.log = log.WithFields(log.Fields{
		types.LogFieldDeliveryID: deliveryID,
		types.LogFieldEventType:  eventType,
	})

	h.log.Infof("Received %s request on %s", r.Method, r.RequestURI)

	if eventType != webhookTypeDeployment {
		return http.StatusNoContent, fmt.Errorf("ignoring unsupported event type '%s'", eventType)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	err = gh.ValidateSignature(sig, data, []byte(h.SecretToken))
	if err != nil {
		return http.StatusForbidden, err
	}

	deploymentEvent := &gh.DeploymentEvent{}
	if err := json.Unmarshal(data, deploymentEvent); err != nil {
		return http.StatusBadRequest, err
	}

	if deploymentEvent.GetDeployment().GetTask() == DirectDeployGithubTask {
		return http.StatusNoContent, fmt.Errorf("ignoring webhook originating from direct deployment through hookd")
	}

	deploymentRequest, err := DeploymentRequestFromEvent(deploymentEvent, deliveryID)

	if err == nil {
		err = h.Clusters.Contains(deploymentRequest.GetCluster())
	}

	if err != nil {
		return http.StatusBadRequest, err
	}

	h.log = h.log.WithFields(deploymentRequest.LogFields())

	if len(deploymentRequest.GetPayloadSpec().GetTeam()) == 0 {
		err := fmt.Errorf("no team was specified in deployment payload")
		h.DeploymentStatus <- *types.NewErrorStatus(*deploymentRequest, err)
		return http.StatusBadRequest, err
	}

	if err := h.validateTeamAccess(deploymentRequest); err != nil {
		h.DeploymentStatus <- *types.NewErrorStatus(*deploymentRequest, err)
		return http.StatusForbidden, err
	}

	h.log.Infof("Validation successful; dispatching deployment")
	h.DeploymentRequest <- *deploymentRequest

	return http.StatusCreated, nil
}
