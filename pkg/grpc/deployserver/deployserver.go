package deployserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/hookd/github"
	log "github.com/sirupsen/logrus"

	"github.com/navikt/deployment/pkg/pb"
)

type DeployServer interface {
	pb.DeployServer
	SendDeploymentRequest(ctx context.Context, deployment pb.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status pb.DeploymentStatus) error
}

type deployServer struct {
	streams      map[string]pb.Deploy_DeploymentsServer
	db           database.DeploymentStore
	githubClient github.Client
	requests     chan pb.DeploymentRequest
	statuses     chan pb.DeploymentStatus
}

func New(db database.DeploymentStore, githubClient github.Client) DeployServer {
	server := &deployServer{
		streams:      make(map[string]pb.Deploy_DeploymentsServer),
		db:           db,
		githubClient: githubClient,
		requests:     make(chan pb.DeploymentRequest, 4096),
		statuses:     make(chan pb.DeploymentStatus, 4096),
	}

	go server.githubLoop()

	return server
}

var _ DeployServer = &deployServer{}

func (s *deployServer) onlineClusters() []string {
	clusters := make([]string, 0, len(s.streams))
	for k := range s.streams {
		clusters = append(clusters, k)
	}
	return clusters
}

func (s *deployServer) Deployments(opts *pb.GetDeploymentOpts, stream pb.Deploy_DeploymentsServer) error {
	err := s.clusterOnline(opts.Cluster)
	if err == nil {
		log.Warnf("Rejected connection from cluster '%s': already connected", opts.Cluster)
		return fmt.Errorf("cluster already connected: %s", opts.Cluster)
	}
	s.streams[opts.Cluster] = stream
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	log.Infof("Online clusters: %s", strings.Join(s.onlineClusters(), ", "))
	<-stream.Context().Done()
	delete(s.streams, opts.Cluster)
	log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
	log.Infof("Online clusters: %s", strings.Join(s.onlineClusters(), ", "))
	return nil
}

func (s *deployServer) ReportStatus(ctx context.Context, status *pb.DeploymentStatus) (*pb.ReportStatusOpts, error) {
	return &pb.ReportStatusOpts{}, s.HandleDeploymentStatus(ctx, *status)
}
