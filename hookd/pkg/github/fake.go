package github

import (
	"context"

	gh "github.com/google/go-github/v27/github"
)

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
