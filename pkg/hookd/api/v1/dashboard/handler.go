package api_v1_dashboard

import (
	"context"
	"net/http"
	"strings"

	"github.com/go-chi/chi"
	"github.com/go-chi/render"
	"github.com/nais/deploy/pkg/hookd/database"
	"github.com/nais/deploy/pkg/hookd/middleware"
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

	// this approach eliminates empty tokens in the final slice
	// e.g. input "myteam," will produce [myteam] and not [myteam ]
	splitFn := func(c rune) bool {
		return c == ','
	}
	teams := strings.FieldsFunc(r.URL.Query().Get("team"), splitFn)

	deployments, err := h.DeploymentStore.Deployments(r.Context(), teams, maxRows)
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
