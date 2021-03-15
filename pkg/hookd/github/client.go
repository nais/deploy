package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v27/github"
	api_v1 "github.com/nais/deploy/pkg/hookd/api/v1"
	"github.com/nais/deploy/pkg/hookd/logproxy"
	"github.com/nais/deploy/pkg/hookd/metrics"
	"github.com/nais/deploy/pkg/pb"
)

var (
	ErrEmptyRepository  = fmt.Errorf("empty repository")
	ErrGitHubNotEnabled = fmt.Errorf("GitHub requests are not enabled")
	ErrTeamNoAccess     = fmt.Errorf("team has no admin access to repository")
	ErrTeamNotExist     = fmt.Errorf("team does not exist on GitHub")
)

const maxDescriptionLength = 140

type Client interface {
	TeamAllowed(ctx context.Context, owner, repository, team string) error
	CreateDeployment(ctx context.Context, request *pb.DeploymentRequest) (*gh.Deployment, error)
	CreateDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus, deploymentID int64) (*gh.DeploymentStatus, error)
}

type client struct {
	client  *gh.Client
	baseurl string
}

func New(c *gh.Client, baseurl string) Client {
	return &client{
		client:  c,
		baseurl: baseurl,
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

	repo, resp, err := c.client.Teams.IsTeamRepo(ctx, team.GetID(), owner, repository)
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

func (c *client) CreateDeployment(ctx context.Context, request *pb.DeploymentRequest) (*gh.Deployment, error) {
	repo := request.GetRepository()
	payload := DeploymentRequest(request)

	dep, resp, err := c.client.Repositories.CreateDeployment(ctx, repo.GetOwner(), repo.GetName(), &payload)

	if resp != nil {
		metrics.GitHubRequest(resp.StatusCode, repo.FullName(), request.GetTeam())
	}

	return dep, err
}

func (c *client) CreateDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus, deploymentID int64) (*gh.DeploymentStatus, error) {
	repo := status.GetRequest().GetRepository()
	if repo == nil {
		return nil, ErrEmptyRepository
	}

	state := status.GetState().String()
	description := status.GetMessage()
	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength]
	}

	url := logproxy.MakeURL(c.baseurl, status.GetRequest().GetID(), time.Now())

	st, resp, err := c.client.Repositories.CreateDeploymentStatus(
		ctx,
		repo.GetOwner(),
		repo.GetName(),
		deploymentID,
		&gh.DeploymentStatusRequest{
			State:       &state,
			Description: &description,
			LogURL:      &url,
		},
	)

	if resp != nil {
		metrics.GitHubRequest(resp.StatusCode, repo.FullName(), status.GetRequest().GetTeam())
	}

	return st, err
}

func DeploymentRequest(r *pb.DeploymentRequest) gh.DeploymentRequest {
	requiredContexts := make([]string, 0)
	return gh.DeploymentRequest{
		Environment:      gh.String(r.GetGithubEnvironment()),
		Ref:              gh.String(r.GetGitRefSha()),
		Task:             gh.String(api_v1.DirectDeployGithubTask),
		AutoMerge:        gh.Bool(false),
		RequiredContexts: &requiredContexts,
	}
}
