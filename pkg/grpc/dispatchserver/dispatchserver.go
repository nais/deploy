package dispatchserver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/hookd/github"
	"github.com/nais/deploy/pkg/hookd/metrics"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nais/deploy/pkg/pb"
)

var maplock sync.Mutex

type DispatchServer interface {
	pb.DispatchServer
	SendDeploymentRequest(ctx context.Context, deployment *pb.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus) error
	StreamStatus(context.Context, chan<- *pb.DeploymentStatus)
}

type dispatchServer struct {
	pb.UnimplementedDispatchServer
	dispatchStreams map[string]pb.Dispatch_DeploymentsServer
	statusStreams   map[context.Context]chan<- *pb.DeploymentStatus
	db              database.DeploymentStore
	githubClient    github.Client
	requests        chan *pb.DeploymentRequest
	statuses        chan *pb.DeploymentStatus
}

func New(db database.DeploymentStore, githubClient github.Client) DispatchServer {
	server := &dispatchServer{
		dispatchStreams: make(map[string]pb.Dispatch_DeploymentsServer),
		statusStreams:   make(map[context.Context]chan<- *pb.DeploymentStatus),
		db:              db,
		githubClient:    githubClient,
		requests:        make(chan *pb.DeploymentRequest, 4096),
		statuses:        make(chan *pb.DeploymentStatus, 4096),
	}

	go server.githubLoop()

	return server
}

var _ DispatchServer = &dispatchServer{}

func (s *dispatchServer) onlineClusters() []string {
	clusters := make([]string, 0, len(s.dispatchStreams))
	for k := range s.dispatchStreams {
		clusters = append(clusters, k)
	}
	return clusters
}

func (s *dispatchServer) reportOnlineClusters() {
	metrics.SetConnectedClusters(s.onlineClusters())
	log.Infof("Online clusters: %s", strings.Join(s.onlineClusters(), ", "))
}

func (s *dispatchServer) invalidateHistoric(ctx context.Context, cluster string, timestamp time.Time) error {
	deploys, err := s.db.HistoricDeployments(ctx, cluster, timestamp)
	if err != nil {
		return err
	}

	for _, deploy := range deploys {
		req := database_mapper.PbRequest(*deploy)
		err = s.HandleDeploymentStatus(ctx, pb.NewInactiveStatus(req))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *dispatchServer) Deployments(opts *pb.GetDeploymentOpts, stream pb.Dispatch_DeploymentsServer) error {
	err := s.clusterOnline(opts.Cluster)
	if err == nil {
		log.Warnf("Rejected connection from cluster '%s': already connected", opts.Cluster)
		return fmt.Errorf("cluster already connected: %s", opts.Cluster)
	}
	maplock.Lock()
	s.dispatchStreams[opts.Cluster] = stream
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	maplock.Unlock()
	s.reportOnlineClusters()

	// invalidate older deployments
	err = s.invalidateHistoric(stream.Context(), opts.GetCluster(), opts.GetStartupTime().AsTime())
	if err != nil {
		return status.Errorf(codes.Unavailable, err.Error())
	}

	// wait for disconnect
	<-stream.Context().Done()

	maplock.Lock()
	delete(s.dispatchStreams, opts.Cluster)
	log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
	maplock.Unlock()
	s.reportOnlineClusters()

	return nil
}

func (s *dispatchServer) ReportStatus(ctx context.Context, status *pb.DeploymentStatus) (*pb.ReportStatusOpts, error) {
	return &pb.ReportStatusOpts{}, s.HandleDeploymentStatus(ctx, status)
}

// Send all status updates belonging to a specific request
func (s *dispatchServer) StreamStatus(ctx context.Context, channel chan<- *pb.DeploymentStatus) {
	maplock.Lock()
	s.statusStreams[ctx] = channel
	maplock.Unlock()

	<-ctx.Done()

	maplock.Lock()
	delete(s.statusStreams, ctx)
	close(channel)
	maplock.Unlock()
}
