package deployserver

import (
	"context"
	"fmt"

	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/github"
	log "github.com/sirupsen/logrus"

	"github.com/navikt/deployment/common/pkg/deployment"
)

type DeployServer interface {
	deployment.DeployServer
	Queue(request *deployment.DeploymentRequest) error
	SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error
}

type deployServer struct {
	streams      map[string]deployment.Deploy_DeploymentsServer
	db           database.DeploymentStore
	githubClient github.Client
	requests     chan deployment.DeploymentRequest
	statuses     chan deployment.DeploymentStatus
}

func New(db database.DeploymentStore, githubClient github.Client) DeployServer {
	server := &deployServer{
		streams:      make(map[string]deployment.Deploy_DeploymentsServer),
		db:           db,
		githubClient: githubClient,
		requests:     make(chan deployment.DeploymentRequest, 4096),
		statuses:     make(chan deployment.DeploymentStatus, 4096),
	}

	go server.githubLoop()

	return server
}

var _ DeployServer = &deployServer{}

func (s *deployServer) Queue(request *deployment.DeploymentRequest) error {
	stream, ok := s.streams[request.Cluster]
	if !ok {
		return fmt.Errorf("cluster '%s' is offline", request.Cluster)
	}
	return stream.Send(request)
}

func (s *deployServer) Deployments(opts *deployment.GetDeploymentOpts, stream deployment.Deploy_DeploymentsServer) error {
	log.Infof("Connection opened from cluster %s", opts.Cluster)
	s.streams[opts.Cluster] = stream
	<-stream.Context().Done()
	delete(s.streams, opts.Cluster)
	log.Errorf("Connection from cluster %s closed", opts.Cluster)
	return nil
}

func (s *deployServer) ReportStatus(ctx context.Context, status *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return &deployment.ReportStatusOpts{}, s.HandleDeploymentStatus(ctx, *status)
}
