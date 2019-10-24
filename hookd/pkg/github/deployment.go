package github

import (
	"context"
	"fmt"

	gh "github.com/google/go-github/v27/github"
)

type Client interface {
	CreateDeployment(ctx context.Context, owner, repository string, request *gh.DeploymentRequest) (*gh.Deployment, error)
}

type client struct {
	client *gh.Client
}

func New(c *gh.Client) Client {
	return &client{
		client: c,
	}
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
