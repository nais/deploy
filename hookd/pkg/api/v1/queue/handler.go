package api_v1_queue

import (
	"net/http"

	"github.com/navikt/deployment/hookd/pkg/metrics"
)

type Handler struct {
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := metrics.WriteQueue(w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
