package deployserver

import (
	"context"
	"fmt"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"

	"github.com/navikt/deployment/common/pkg/deployment"
)

const channelSize = 1000

type DeployServer interface {
	deployment.DeployServer
	Queue(request *deployment.DeploymentRequest)
	SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error
}

type deployServer struct {
	channels     map[string]chan *deployment.DeploymentRequest
	db           database.DeploymentStore
	githubClient github.Client
	requests     chan deployment.DeploymentRequest
	statuses     chan deployment.DeploymentStatus
}

func New(clusters []string, db database.DeploymentStore, githubClient github.Client) DeployServer {
	server := &deployServer{
		channels:     make(map[string]chan *deployment.DeploymentRequest),
		db:           db,
		githubClient: githubClient,
		requests:     make(chan deployment.DeploymentRequest, 4096),
		statuses:     make(chan deployment.DeploymentStatus, 4096),
	}
	for _, cluster := range clusters {
		server.channels[cluster] = make(chan *deployment.DeploymentRequest, channelSize)
	}

	go server.githubLoop()

	return server
}

var _ DeployServer = &deployServer{}

func (s *deployServer) Queue(request *deployment.DeploymentRequest) {
	s.channels[request.Cluster] <- request
}

func (s *deployServer) Deployments(deploymentOpts *deployment.GetDeploymentOpts, deploymentsServer deployment.Deploy_DeploymentsServer) error {
	for message := range s.channels[deploymentOpts.Cluster] {
		err := deploymentsServer.Send(message)
		if err != nil {
			return fmt.Errorf("unable to send deployment message: %w", err)
		}
	}
	return fmt.Errorf("channel closed unexpectedly")
}

func (s *deployServer) ReportStatus(ctx context.Context, status *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, s.HandleDeploymentStatus(ctx, *status)
}
