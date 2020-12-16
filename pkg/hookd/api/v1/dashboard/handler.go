package api_v1_dashboard

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/hookd/middleware"
	log "github.com/sirupsen/logrus"
)

const (
	maxRows = 30
)

type DeploymentsResponse struct {
	Deployments []*database.Deployment `json:"deployments"`
}

type FullDeployment struct {
	Deployment database.Deployment         `json:"deployment"`
	Statuses   []database.DeploymentStatus `json:"statuses"`
}

type Handler struct {
	DeploymentStore database.DeploymentStore
}

func (h *Handler) Deployments(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)

	deployments, err := h.DeploymentStore.Deployments(r.Context(), "", maxRows)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	render.JSON(w, r, DeploymentsResponse{
		Deployments: deployments,
	})
}

func (h *Handler) Deployment(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	deploymentID := chi.URLParam(r, "id")

	deployment, err := h.DeploymentStore.Deployment(r.Context(), deploymentID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	statuses, err := h.DeploymentStore.DeploymentStatus(r.Context(), deploymentID)
	if err != nil && err != database.ErrNotFound {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	render.JSON(w, r, FullDeployment{
		Deployment: *deployment,
		Statuses:   statuses,
	})
}
