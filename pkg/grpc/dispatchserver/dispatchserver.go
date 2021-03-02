package dispatchserver

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/hookd/github"
	"github.com/navikt/deployment/pkg/hookd/metrics"
	log "github.com/sirupsen/logrus"

	"github.com/navikt/deployment/pkg/pb"
)

var maplock sync.Mutex

type DispatchServer interface {
	pb.DispatchServer
	SendDeploymentRequest(ctx context.Context, deployment *pb.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus) error
}

type dispatchServer struct {
	pb.UnimplementedDispatchServer
	streams      map[string]pb.Dispatch_DeploymentsServer
	db           database.DeploymentStore
	githubClient github.Client
	requests     chan *pb.DeploymentRequest
	statuses     chan *pb.DeploymentStatus
}

func New(db database.DeploymentStore, githubClient github.Client) DispatchServer {
	server := &dispatchServer{
		streams:      make(map[string]pb.Dispatch_DeploymentsServer),
		db:           db,
		githubClient: githubClient,
		requests:     make(chan *pb.DeploymentRequest, 4096),
		statuses:     make(chan *pb.DeploymentStatus, 4096),
	}

	go server.githubLoop()

	return server
}

var _ DispatchServer = &dispatchServer{}

func (s *dispatchServer) onlineClusters() []string {
	clusters := make([]string, 0, len(s.streams))
	for k := range s.streams {
		clusters = append(clusters, k)
	}
	return clusters
}

func (s *dispatchServer) reportOnlineClusters() {
	metrics.SetConnectedClusters(s.onlineClusters())
	log.Infof("Online clusters: %s", strings.Join(s.onlineClusters(), ", "))
}

func (s *dispatchServer) Deployments(opts *pb.GetDeploymentOpts, stream pb.Dispatch_DeploymentsServer) error {
	err := s.clusterOnline(opts.Cluster)
	if err == nil {
		log.Warnf("Rejected connection from cluster '%s': already connected", opts.Cluster)
		return fmt.Errorf("cluster already connected: %s", opts.Cluster)
	}
	maplock.Lock()
	s.streams[opts.Cluster] = stream
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	maplock.Unlock()
	s.reportOnlineClusters()

	// wait for disconnect
	<-stream.Context().Done()

	maplock.Lock()
	delete(s.streams, opts.Cluster)
	log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
	maplock.Unlock()
	s.reportOnlineClusters()

	return nil
}

func (s *dispatchServer) ReportStatus(ctx context.Context, status *pb.DeploymentStatus) (*pb.ReportStatusOpts, error) {
	return &pb.ReportStatusOpts{}, s.HandleDeploymentStatus(ctx, status)
}
