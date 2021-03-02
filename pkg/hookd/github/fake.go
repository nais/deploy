package github

import (
	"context"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/pkg/pb"
)

type fakeClient struct{}

func FakeClient() Client {
	return &fakeClient{}
}

func (c *fakeClient) CreateDeployment(ctx context.Context, request *pb.DeploymentRequest) (*gh.Deployment, error) {
	return nil, ErrGitHubNotEnabled
}

func (c *fakeClient) TeamAllowed(ctx context.Context, owner, repository, team string) error {
	return ErrGitHubNotEnabled
}

func (c *fakeClient) CreateDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus, deploymentID int64) (*gh.DeploymentStatus, error) {
	return nil, ErrGitHubNotEnabled
}
