package server

import (
	"encoding/json"
	"fmt"
	gh "github.com/google/go-github/v23/github"
	"net/http"
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
	h.log.Infof("Handling deployment event webhook")
	h.log.Infof("Dispatching deployment for %s", h.repo.GetFullName())

	return http.StatusCreated, nil
}
