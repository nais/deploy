package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v23/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	webhookTypeDeployment = "deployment"
)

type DeploymentHandler struct {
	log                      *log.Entry
	Config                   config.Config
	KafkaClient              *kafka.DualClient
	KafkaTopic               string
	GithubInstallationClient *gh.Client
	SecretToken              string
	TeamRepositoryStorage    persistence.TeamRepositoryStorage
}

func (h *DeploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	code, err := h.handler(r)

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

func (h *DeploymentHandler) kafkaPublish(req *types.DeploymentRequest) error {
	payload, err := types.WrapMessage(req, h.KafkaClient.SignatureKey)
	if err != nil {
		return fmt.Errorf("while marshalling json: %s", err)
	}
	msg := sarama.ProducerMessage{
		Topic:     h.KafkaTopic,
		Value:     sarama.StringEncoder(payload),
		Timestamp: time.Unix(req.GetTimestamp(), 0),
	}
	_, _, err = h.KafkaClient.Producer.SendMessage(&msg)
	if err != nil {
		return fmt.Errorf("while publishing message to Kafka: %s", err)
	}
	return nil
}

func (h *DeploymentHandler) createAndLogDeploymentStatus(st *types.DeploymentStatus) error {
	status, _, err := github.CreateDeploymentStatus(h.GithubInstallationClient, st, h.Config.BaseURL)
	if err == nil {
		h.log.Infof("created GitHub deployment status %d in repository %s", status.GetID(), status.GetRepositoryURL())
	}
	return err
}

func (h *DeploymentHandler) addGithubStatusFailure(req *types.DeploymentRequest, err error) error {
	return h.createAndLogDeploymentStatus(&types.DeploymentStatus{
		Deployment:  req.GetDeployment(),
		DeliveryID:  req.GetDeliveryID(),
		State:       types.GithubDeploymentState_failure,
		Description: fmt.Sprintf("deployment request failed: %s", err),
	})
}

func (h *DeploymentHandler) addGithubStatusQueued(req *types.DeploymentRequest) error {
	return h.createAndLogDeploymentStatus(&types.DeploymentStatus{
		Deployment:  req.GetDeployment(),
		DeliveryID:  req.GetDeliveryID(),
		State:       types.GithubDeploymentState_queued,
		Description: "deployment request has been put on the queue for further processing",
	})
}

func (h *DeploymentHandler) validateTeamAccess(req *types.DeploymentRequest) error {
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

func (h *DeploymentHandler) handler(r *http.Request) (int, error) {
	var err error

	deliveryID := r.Header.Get("X-GitHub-Delivery")
	eventType := r.Header.Get("X-GitHub-Event")
	sig := r.Header.Get("X-Hub-Signature")

	h.log = log.WithFields(log.Fields{
		"delivery_id": deliveryID,
		"event_type":  eventType,
	})

	h.log.Infof("Received %s request on %s", r.Method, r.RequestURI)

	metrics.WebhookRequests.Inc()

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

	deploymentRequest, err := DeploymentRequest(deploymentEvent, deliveryID)

	h.log = h.log.WithFields(log.Fields{
		"deployment_id": deploymentRequest.GetDeployment().GetDeploymentID(),
		"team":          deploymentRequest.GetPayloadSpec().GetTeam(),
		"cluster":       deploymentRequest.GetCluster(),
		"repository":    deploymentEvent.GetRepo().GetFullName(),
	})

	if err != nil {
		return http.StatusBadRequest, err
	}

	if err := h.validateTeamAccess(deploymentRequest); err != nil {
		return http.StatusForbidden, err
	}

	h.log.Infof("Validation successful; dispatching deployment")

	err = h.kafkaPublish(deploymentRequest)

	if err != nil {
		erro := h.addGithubStatusFailure(deploymentRequest, fmt.Errorf("unable to queue deployment request to Kafka"))
		if erro != nil {
			h.log.Errorf("unable to create Github deployment status: %s", erro)
		}
		return http.StatusInternalServerError, err
	}

	metrics.Dispatched.Inc()

	err = h.addGithubStatusQueued(deploymentRequest)

	if err != nil {
		h.log.Errorf("unable to create Github deployment status: %s", err)
	}

	return http.StatusCreated, nil
}
