package api_v1_dashboard

import (
	"context"
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
	Deployments []FullDeployment `json:"deployments"`
}

type FullDeployment struct {
	Deployment database.Deployment           `json:"deployment"`
	Statuses   []database.DeploymentStatus   `json:"statuses"`
	Resources  []database.DeploymentResource `json:"resources"`
}

type Handler struct {
	DeploymentStore database.DeploymentStore
}

func (h *Handler) fullDeployment(ctx context.Context, deploymentID string) (*FullDeployment, error) {
	deployment, err := h.DeploymentStore.Deployment(ctx, deploymentID)
	if err != nil {
		return nil, err
	}

	statuses, err := h.DeploymentStore.DeploymentStatus(ctx, deploymentID)
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}

	resources, err := h.DeploymentStore.DeploymentResources(ctx, deploymentID)
	if err != nil && err != database.ErrNotFound {
		return nil, err
	}

	return &FullDeployment{
		Deployment: *deployment,
		Statuses:   statuses,
		Resources:  resources,
	}, nil
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

	fullDeploys := make([]FullDeployment, len(deployments))

	for i := range deployments {
		fd, err := h.fullDeployment(r.Context(), deployments[i].ID)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			logger.Error(err)
			return
		}
		fullDeploys[i] = *fd
	}

	render.JSON(w, r, DeploymentsResponse{
		Deployments: fullDeploys,
	})
}

func (h *Handler) Deployment(w http.ResponseWriter, r *http.Request) {
	fields := middleware.RequestLogFields(r)
	logger := log.WithFields(fields)
	deploymentID := chi.URLParam(r, "id")
	fd, err := h.fullDeployment(r.Context(), deploymentID)

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		logger.Error(err)
		return
	}

	render.JSON(w, r, *fd)
}
