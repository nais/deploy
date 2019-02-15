package server

import (
	"encoding/json"
	"fmt"
	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	"net/http"
	"strconv"
)

type LifecycleHandler struct {
	Handler
	installRequest *gh.InstallationRepositoriesEvent
}

func (h *LifecycleHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := h.prepare(w, r, h.unserialize, h.secretToken); err != nil {
		h.log.Error(err)
		return
	}
	h.finish(h.handler())
}

func (h *LifecycleHandler) unserialize() error {
	h.installRequest = &gh.InstallationRepositoriesEvent{}
	if err := json.Unmarshal(h.data, h.installRequest); err != nil {
		return fmt.Errorf("while decoding JSON data: %s", err)
	}
	return nil
}

func (h *LifecycleHandler) secretToken() (string, error) {
	return h.SecretClient.ApplicationSecret()
}

func (h *LifecycleHandler) handler() (int, error) {
	var err error

	if h.eventType != "installation_repositories" {
		return http.StatusBadRequest, fmt.Errorf("unsupported event type %s", h.eventType)
	}

	switch h.installRequest.GetAction() {
	case "added":
		err = h.handleAddedRepositories()
	case "removed":
		err = h.handleRemovedRepositories()
	default:
		return http.StatusBadRequest, fmt.Errorf("unknown installation action %s", h.installRequest.GetAction())
	}

	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusNoContent, nil
}

func (h *LifecycleHandler) handleAddedRepository(repo *gh.Repository, installation *gh.Installation) error {
	name := repo.GetFullName()

	h.log.Infof("Installing configuration for repository %s", name)

	if installation == nil {
		return fmt.Errorf("empty installation object for %s, cannot install webhook", name)
	}

	id := int(installation.GetID())
	client, err := github.InstallationClient(h.Config.ApplicationID, id, h.Config.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github client for %s: %s", name, err)
	}

	secret, err := secrets.RandomString(32)
	if err != nil {
		return fmt.Errorf("cannot generate random secret string: %s", err)
	}

	hook, err := github.CreateHook(client, *repo, h.Config.WebhookURL, secret)
	if err != nil {
		return fmt.Errorf("while creating webhook: %s", err)
	}

	err = h.SecretClient.WriteInstallationSecret(secrets.InstallationSecret{
		Repository:     name,
		WebhookID:      fmt.Sprintf("%d", hook.GetID()),
		WebhookSecret:  secret,
		InstallationID: strconv.Itoa(id),
	})

	if err != nil {
		return fmt.Errorf("while persisting repository secret: %s", err)
	}

	h.log.Infof("Created webhook in repository %s with id %d", name, hook.GetID())

	return nil
}

func (h *LifecycleHandler) handleRemovedRepository(repo *gh.Repository, installation *gh.Installation) error {
	name := repo.GetFullName()

	h.log.Infof("Removing configuration for repository %s", name)

	if installation == nil {
		return fmt.Errorf("empty installation object for %s, cannot remove data", name)
	}

	// At this point, we would really like to delete the webhook that the integration
	// automatically created upon registration. Unfortunately, we have already lost access.

	/*
	id := int(installation.GetID())
	client, err := github.InstallationClient(config.ApplicationID, id, config.KeyFile)
	if err != nil {
		return fmt.Errorf("cannot instantiate Github client for %s: %s", name, err)
	}

	secret, err := secretClient.InstallationSecret(name)
	if err != nil {
		return fmt.Errorf("unable to retrieve pre-shared secret for repository '%s'", name)
	}

	webhookID, _ := strconv.ParseInt(secret.WebhookID, 10, 64)
	err = github.DeleteHook(client, *repo, webhookID)
	if err != nil {
		return fmt.Errorf("while deleting webhook: %s", err)
	}

	h.log.Infof("deleted webhook in repository %s with id %d", name, webhookID)
	*/

	err := h.SecretClient.DeleteInstallationSecret(name)
	if err != nil {
		return fmt.Errorf("while deleting repository secret: %s", err)
	}

	return nil
}

func (h *LifecycleHandler) handleAddedRepositories() error {
	for _, repo := range h.installRequest.RepositoriesAdded {
		installation := h.installRequest.GetInstallation()
		if err := h.handleAddedRepository(repo, installation); err != nil {
			return err
		}
	}
	return nil
}

func (h *LifecycleHandler) handleRemovedRepositories() error {
	for _, repo := range h.installRequest.RepositoriesRemoved {
		installation := h.installRequest.GetInstallation()
		if err := h.handleRemovedRepository(repo, installation); err != nil {
			return err
		}
	}
	return nil
}
