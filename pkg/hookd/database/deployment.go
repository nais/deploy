package database

import (
	"context"
	"time"
)

type Deployment struct {
	ID               string
	Team             string
	Created          time.Time
	GitHubID         *int
	GitHubRepository *string
	GitRefSha        *string
}

type DeploymentStatus struct {
	ID           string
	DeploymentID string
	Status       string
	Message      string
	Created      time.Time
}

type DeploymentStore interface {
	Deployment(ctx context.Context, id string) (*Deployment, error)
	WriteDeployment(ctx context.Context, deployment Deployment) error
	DeploymentStatus(ctx context.Context, deploymentID string) ([]DeploymentStatus, error)
	WriteDeploymentStatus(ctx context.Context, status DeploymentStatus) error
}

var _ DeploymentStore = &database{}

func (db *database) Deployment(ctx context.Context, id string) (*Deployment, error) {
	query := `SELECT id, team, created, github_id, github_repository FROM deployment WHERE id = $1;`
	rows, err := db.timedQuery(ctx, query, id)

	if err != nil {
		return nil, err
	}

	defer rows.Close()
	for rows.Next() {
		deployment := &Deployment{}

		err := rows.Scan(
			&deployment.ID,
			&deployment.Team,
			&deployment.Created,
			&deployment.GitHubID,
			&deployment.GitHubRepository,
		)

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
