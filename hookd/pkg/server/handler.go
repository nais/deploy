package server

import (
	"fmt"
	"io/ioutil"
	"net/http"

	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/persistence"
	log "github.com/sirupsen/logrus"
)

const (
	webhookTypeDeployment = "deployment"
)

type Handler struct {
	w                        http.ResponseWriter
	r                        *http.Request
	data                     []byte
	log                      *log.Entry
	eventType                string
	deliveryID               string
	Config                   config.Config
	KafkaClient              *kafka.DualClient
	KafkaTopic               string
	GithubInstallationClient *gh.Client
	SecretToken              string
	TeamRepositoryStorage    persistence.TeamRepositoryStorage
}

func (h *Handler) prepare(w http.ResponseWriter, r *http.Request, unserialize func() error, secretToken string) error {
	var err error

	h.deliveryID = r.Header.Get("X-GitHub-Delivery")
	h.eventType = r.Header.Get("X-GitHub-Event")
	h.w = w
	h.r = r

	h.log = log.WithFields(log.Fields{
		"delivery_id": h.deliveryID,
		"event_type":  h.eventType,
	})
	h.log.Infof("received %s request on %s", r.Method, r.RequestURI)

	h.data, err = ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	if h.eventType != webhookTypeDeployment {
		w.WriteHeader(http.StatusNoContent)
		return fmt.Errorf("ignoring unsupported event type '%s'", h.eventType)
	}

	err = unserialize()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	sig := h.r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, h.data, []byte(secretToken))
	if err != nil {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("invalid payload signature: %s", err)
	}

	return nil
}

func (h *Handler) finish(statusCode int, err error) {
	if err != nil {
		h.log.Errorf("%s", err)
		return
	}

	h.w.WriteHeader(statusCode)

	h.log.Infof("Finished handling request")
}
