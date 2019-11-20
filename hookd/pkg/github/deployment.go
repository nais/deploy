package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v27/github"
)

var (
	ErrTeamNotExist         = fmt.Errorf("team does not exist on GitHub")
	ErrTeamNoAccess         = fmt.Errorf("team has no admin access to repository")
	ErrDeploymentNotFound   = fmt.Errorf("deployment not found")
	ErrNoDeploymentStatuses = fmt.Errorf("no deployment statuses available")
	ErrGitHubNotEnabled     = fmt.Errorf("GitHub requests are not enabled")
)

type Client interface {
	CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error)
	TeamAllowed(ctx context.Context, owner, repository, team string) error
	DeploymentStatus(ctx context.Context, owner, repository string, deploymentID int64) (*gh.DeploymentStatus, error)
}

type client struct {
	client *gh.Client
}

func New(c *gh.Client) Client {
	return &client{
		client: c,
	}
}

func (c *client) TeamAllowed(ctx context.Context, owner, repository, teamName string) error {
	team, resp, err := c.client.Teams.GetTeamBySlug(ctx, owner, teamName)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return ErrTeamNotExist
		}
		return err
	}

	repo, _, err := c.client.Teams.IsTeamRepo(ctx, team.GetID(), owner, repository)
	if err != nil {
		if resp != nil && resp.StatusCode == http.StatusNotFound {
			return ErrTeamNoAccess
		}
		return err
	}

	if repo == nil {
		return ErrTeamNoAccess
	}

	return nil
}

func (c *client) CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error) {
	deployment, _, err := c.client.Repositories.CreateDeployment(ctx, owner, repository, request)
	return deployment, err
}

func (c *client) DeploymentStatus(ctx context.Context, owner, repository string, deploymentID int64) (*gh.DeploymentStatus, error) {
	opts := &gh.ListOptions{
		PerPage: 1,
	}
	deploy, resp, err := c.client.Repositories.ListDeploymentStatuses(ctx, owner, repository, deploymentID, opts)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		return nil, ErrDeploymentNotFound
	}
	if err != nil {
		return nil, err
	}
	if len(deploy) == 0 {
		return nil, ErrNoDeploymentStatuses
	}
	return deploy[0], nil
}

type fakeClient struct{}

func FakeClient() Client {
	return &fakeClient{}
}

func (c *fakeClient) CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error) {
	return nil, ErrGitHubNotEnabled
}

func (c *fakeClient) TeamAllowed(ctx context.Context, owner, repository, team string) error {
	return ErrGitHubNotEnabled
}

func (c *fakeClient) DeploymentStatus(ctx context.Context, owner, repository string, deploymentID int64) (*gh.DeploymentStatus, error) {
	return nil, ErrGitHubNotEnabled
}
