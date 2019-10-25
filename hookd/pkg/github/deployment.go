package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v27/github"
)

var (
	ErrTeamNotExist = fmt.Errorf("team does not exist on GitHub")
	ErrTeamNoAccess = fmt.Errorf("team has no admin access to repository")
)

type Client interface {
	CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error)
	TeamAllowed(ctx context.Context, owner, repository, team string) error
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

type fakeClient struct{}

func FakeClient() Client {
	return &fakeClient{}
}

func (c *fakeClient) CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error) {
	return nil, fmt.Errorf("GitHub requests are not enabled")
}

func (c *fakeClient) TeamAllowed(ctx context.Context, owner, repository, team string) error {
	return fmt.Errorf("GitHub requests are not enabled")
}
