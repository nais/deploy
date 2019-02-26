package server

import (
	"fmt"
	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/config"
	"github.com/navikt/deployment/hookd/pkg/secrets"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

type Handler struct {
	w                        http.ResponseWriter
	r                        *http.Request
	data                     []byte
	log                      *log.Entry
	eventType                string
	deliveryID               string
	Config                   config.Config
	SecretClient             *secrets.Client
	KafkaProducer            sarama.SyncProducer
	KafkaTopic               string
	GithubClient             *gh.Client
	GithubInstallationClient *gh.Client
}

func (h *Handler) prepare(w http.ResponseWriter, r *http.Request, unserialize func() error, secretToken func() (string, error)) error {
	var err error

	h.deliveryID = r.Header.Get("X-GitHub-Delivery")
	h.eventType = r.Header.Get("X-GitHub-Event")
	h.w = w
	h.r = r

	h.log = log.WithFields(log.Fields{
		"delivery_id": h.deliveryID,
		"event_type":  h.eventType,
	})

	h.log.Infof("%s %s %s", r.Method, r.RequestURI, h.eventType)

	h.data, err = ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	if h.eventType == "ping" {
		w.WriteHeader(http.StatusNoContent)
		return fmt.Errorf("received ping request")
	}

	err = unserialize()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	psk, err := secretToken()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	sig := h.r.Header.Get("X-Hub-Signature")
	err = gh.ValidateSignature(sig, h.data, []byte(psk))
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
