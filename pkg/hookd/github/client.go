package github

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/pkg/pb"
	api_v1 "github.com/navikt/deployment/pkg/hookd/api/v1"
	"github.com/navikt/deployment/pkg/hookd/logproxy"
	"github.com/navikt/deployment/pkg/hookd/metrics"
)

var (
	ErrEmptyDeployment  = fmt.Errorf("empty deployment")
	ErrEmptyRepository  = fmt.Errorf("empty repository")
	ErrGitHubNotEnabled = fmt.Errorf("GitHub requests are not enabled")
	ErrTeamNoAccess     = fmt.Errorf("team has no admin access to repository")
	ErrTeamNotExist     = fmt.Errorf("team does not exist on GitHub")
)

const maxDescriptionLength = 140

type Client interface {
	TeamAllowed(ctx context.Context, owner, repository, team string) error
	CreateDeployment(ctx context.Context, request pb.DeploymentRequest) (*gh.Deployment, error)
	CreateDeploymentStatus(ctx context.Context, status pb.DeploymentStatus) (*gh.DeploymentStatus, error)
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

func (c *client) CreateDeployment(ctx context.Context, request pb.DeploymentRequest) (*gh.Deployment, error) {
	repo := request.GetDeployment().GetRepository()
	payload := DeploymentRequest(request)

	dep, resp, err := c.client.Repositories.CreateDeployment(ctx, repo.GetOwner(), repo.GetName(), &payload)

	if resp != nil {
		metrics.GitHubRequest(resp.StatusCode, repo.FullName(), request.GetPayloadSpec().GetTeam())
	}

	return dep, err
}

func (c *client) CreateDeploymentStatus(ctx context.Context, status pb.DeploymentStatus) (*gh.DeploymentStatus, error) {
	dep := status.GetDeployment()
	if dep == nil {
		return nil, ErrEmptyDeployment
	}

	repo := dep.GetRepository()
	if repo == nil {
		return nil, ErrEmptyRepository
	}

	state := status.GetState().String()
	description := status.GetDescription()
	if len(description) > maxDescriptionLength {
		description = description[:maxDescriptionLength]
	}

	url := logproxy.MakeURL(c.baseurl, status.GetDeliveryID(), time.Now())

	st, resp, err := c.client.Repositories.CreateDeploymentStatus(
		ctx,
		repo.GetOwner(),
		repo.GetName(),
		dep.GetDeploymentID(),
		&gh.DeploymentStatusRequest{
			State:       &state,
			Description: &description,
			LogURL:      &url,
		},
	)

	if resp != nil {
		metrics.GitHubRequest(resp.StatusCode, repo.FullName(), status.GetTeam())
	}

	return st, err
}

func DeploymentRequest(r pb.DeploymentRequest) gh.DeploymentRequest {
	requiredContexts := make([]string, 0)
	return gh.DeploymentRequest{
		Environment:      gh.String(r.GetDeployment().GetEnvironment()),
		Ref:              gh.String(r.GetDeployment().GetRef()),
		Task:             gh.String(api_v1.DirectDeployGithubTask),
		AutoMerge:        gh.Bool(false),
		RequiredContexts: &requiredContexts,
	}
}
