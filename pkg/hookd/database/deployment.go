package database

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4"
)

type Deployment struct {
	ID               string    `json:"id"`
	Team             string    `json:"team"`
	Created          time.Time `json:"created"`
	GitHubID         *int      `json:"githubID"`
	GitHubRepository *string   `json:"githubRepository"`
}

type DeploymentStatus struct {
	ID           string    `json:"id"`
	DeploymentID string    `json:"deploymentID"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	Created      time.Time `json:"created"`
}

type DeploymentResource struct {
	ID           string `json:"id"`
	DeploymentID string `json:"deploymentID"`
	Index        int    `json:"index"`
	Group        string `json:"group"`
	Version      string `json:"version"`
	Kind         string `json:"kind"`
	Name         string `json:"name"`
	Namespace    string `json:"namespace"`
}

type DeploymentStore interface {
	Deployments(ctx context.Context, team string, limit int) ([]*Deployment, error)
	Deployment(ctx context.Context, id string) (*Deployment, error)
	WriteDeployment(ctx context.Context, deployment Deployment) error
	DeploymentStatus(ctx context.Context, deploymentID string) ([]DeploymentStatus, error)
	WriteDeploymentStatus(ctx context.Context, status DeploymentStatus) error
	DeploymentResources(ctx context.Context, deploymentID string) ([]DeploymentResource, error)
	WriteDeploymentResource(ctx context.Context, resource DeploymentResource) error
}

var _ DeploymentStore = &database{}

func scanDeployment(rows pgx.Rows) (*Deployment, error) {
	deployment := &Deployment{}

	err := rows.Scan(
		&deployment.ID,
		&deployment.Team,
		&deployment.Created,
		&deployment.GitHubID,
		&deployment.GitHubRepository,
	)

	return deployment, err
}

func (db *database) Deployments(ctx context.Context, team string, limit int) ([]*Deployment, error) {
	query := `
SELECT id, team, created, github_id, github_repository
FROM deployment
WHERE ($1 = '' OR team = $1)
ORDER BY created DESC
LIMIT $2;
`
	rows, err := db.timedQuery(ctx, query, team, limit)

	if err != nil {
		return nil, err
	}

	deployments := make([]*Deployment, 0)
	defer rows.Close()
	for rows.Next() {
		deployment, err := scanDeployment(rows)

		if err != nil {
			return nil, err
		}

		deployments = append(deployments, deployment)
	}

	return deployments, nil
}

func (db *database) Deployment(ctx context.Context, id string) (*Deployment, error) {
	query := `SELECT id, team, created, github_id, github_repository FROM deployment WHERE id = $1;`
	rows, err := db.timedQuery(ctx, query, id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		deployment, err := scanDeployment(rows)

		if err != nil {
			return nil, err
		}

		return deployment, nil
	}

	return nil, ErrNotFound
}

func (db *database) WriteDeployment(ctx context.Context, deployment Deployment) error {
	var query string

	query = `
INSERT INTO deployment (id, team, created, github_id, github_repository)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (id) DO UPDATE
SET github_id = EXCLUDED.github_id, github_repository = EXCLUDED.github_repository;
`
	_, err := db.conn.Exec(ctx, query,
		deployment.ID,
		deployment.Team,
		deployment.Created,
		deployment.GitHubID,
		deployment.GitHubRepository,
	)

	return err
}

func (db *database) DeploymentStatus(ctx context.Context, deploymentID string) ([]DeploymentStatus, error) {
	query := `SELECT id, deployment_id, status, message, created FROM deployment_status WHERE deployment_id = $1 ORDER BY created DESC;`
	rows, err := db.timedQuery(ctx, query, deploymentID)

	if err != nil {
		return nil, err
	}

	statuses := make([]DeploymentStatus, 0)

	defer rows.Close()
	for rows.Next() {
		status := DeploymentStatus{}

		// see selectApiKeyFields
		err := rows.Scan(
			&status.ID,
			&status.DeploymentID,
			&status.Status,
			&status.Message,
			&status.Created,
		)

		if err != nil {
			return nil, err
		}

		statuses = append(statuses, status)
	}

	if len(statuses) == 0 {
		return nil, ErrNotFound
	}

	return statuses, nil
}

func (db *database) WriteDeploymentStatus(ctx context.Context, status DeploymentStatus) error {
	var query string

	query = `
INSERT INTO deployment_status (id, deployment_id, status, message, created)
VALUES ($1, $2, $3, $4, $5);
`
	_, err := db.conn.Exec(ctx, query,
		status.ID,
		status.DeploymentID,
		status.Status,
		status.Message,
		status.Created,
	)

	return err
}

func (db *database) DeploymentResources(ctx context.Context, deploymentID string) ([]DeploymentResource, error) {
	query := `SELECT id, deployment_id, index, "group", version, kind, name, namespace FROM deployment_resource WHERE deployment_id = $1 ORDER BY index ASC;`
	rows, err := db.timedQuery(ctx, query, deploymentID)

	if err != nil {
		return nil, err
	}

	resources := make([]DeploymentResource, 0)

	defer rows.Close()
	for rows.Next() {
		resource := DeploymentResource{}

		// see selectApiKeyFields
		err := rows.Scan(
			&resource.ID,
			&resource.DeploymentID,
			&resource.Index,
			&resource.Group,
			&resource.Version,
			&resource.Kind,
			&resource.Name,
			&resource.Namespace,
		)

		if err != nil {
			return nil, err
		}

		resources = append(resources, resource)
	}

	return resources, nil
}

func (db *database) WriteDeploymentResource(ctx context.Context, resource DeploymentResource) error {
	var query string

	query = `
INSERT INTO deployment_resource (id, deployment_id, index, "group", version, kind, name, namespace)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8);
`
	_, err := db.conn.Exec(ctx, query,
		resource.ID,
		resource.DeploymentID,
		resource.Index,
		resource.Group,
		resource.Version,
		resource.Kind,
		resource.Name,
		resource.Namespace,
	)

	return err
}
