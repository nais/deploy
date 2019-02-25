package server

import (
	"encoding/json"
	"fmt"
	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v23/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	proto "github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/hookd/pkg/github"
	"net/http"
	"time"
)

type DeploymentHandler struct {
	Handler
	deploymentRequest *gh.DeploymentEvent
	repo              *gh.Repository
}

func (h *DeploymentHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.prepare(w, r, h.unserialize, h.secretToken); err != nil {
		h.log.Error(err)
		return
	}
	h.finish(h.handler())
}

func (h *DeploymentHandler) kafkaPublish() error {
	owner, name, err := github.SplitFullname(h.repo.GetFullName())
	if err != nil {
		return err
	}
	deployment := h.deploymentRequest.GetDeployment()
	if deployment == nil {
		return fmt.Errorf("deployment object is empty")
	}
	deploymentRequest := &types.DeploymentRequest{
		Deployment: &types.DeploymentSpec{
			Repository: &types.GithubRepository{
				Name:  name,
				Owner: owner,
			},
			DeploymentID: deployment.GetID(),
		},
		CorrelationID: h.deliveryID,
		Cluster:       deployment.GetEnvironment(),
		Timestamp:     time.Now().Unix(),
		Deadline:      time.Now().Add(time.Minute).Unix(),
	}

	payload, err := proto.Marshal(deploymentRequest)
	if err != nil {
		return fmt.Errorf("while marshalling json: %s", err)
	}
	msg := sarama.ProducerMessage{
		Topic: h.KafkaTopic,
		Value: sarama.StringEncoder(payload),
	}
	_, _, err = h.KafkaProducer.SendMessage(&msg)
	if err != nil {
		return fmt.Errorf("while publishing message to Kafka: %s", err)
	}
	return nil
}

func (h *DeploymentHandler) unserialize() error {
	h.deploymentRequest = &gh.DeploymentEvent{}
	if err := json.Unmarshal(h.data, h.deploymentRequest); err != nil {
		return err
	}
	h.repo = h.deploymentRequest.GetRepo()
	if h.repo == nil {
		return fmt.Errorf("deployment request doesn't specify repository")
	}
	return nil
}

func (h *DeploymentHandler) secretToken() (string, error) {
	deploymentRequest := gh.DeploymentEvent{}
	if err := json.Unmarshal(h.data, &deploymentRequest); err != nil {
		return "", err
	}
	repo := deploymentRequest.GetRepo()
	if repo == nil {
		return "", fmt.Errorf("deployment request doesn't specify repository")
	}
	secret, err := h.SecretClient.InstallationSecret(repo.GetFullName())
	if err != nil {
		return "", err
	}
	return secret.WebhookSecret, nil
}

func (h *DeploymentHandler) handler() (int, error) {
	if h.eventType != "deployment" {
		return http.StatusBadRequest, fmt.Errorf("unsupported event type %s", h.eventType)
	}

	h.log.Infof("Dispatching deployment for %s", h.repo.GetFullName())
	err := h.kafkaPublish()

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusCreated, nil
}
