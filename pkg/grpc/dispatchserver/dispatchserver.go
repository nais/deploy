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

type DispatchServer interface {
	pb.DispatchServer
	SendDeploymentRequest(ctx context.Context, deployment *pb.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus) error
	StreamStatus(context.Context, chan<- *pb.DeploymentStatus)
}

type dispatchServer struct {
	pb.UnimplementedDispatchServer
	dispatchStreamsLock sync.RWMutex
	dispatchStreams     map[string]pb.Dispatch_DeploymentsServer
	statusStreamsLock   sync.RWMutex
	statusStreams       map[context.Context]chan<- *pb.DeploymentStatus
	db                  database.DeploymentStore
	githubClient        github.Client
	requests            chan *pb.DeploymentRequest
	statuses            chan *pb.DeploymentStatus
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
	s.dispatchStreamsLock.RLock()
	defer s.dispatchStreamsLock.RUnlock()

	clusters := make([]string, 0, len(s.dispatchStreams))
	for k := range s.dispatchStreams {
		clusters = append(clusters, k)
	}
	return clusters
}

func (s *dispatchServer) reportOnlineClusters() {
	clusters := s.onlineClusters()
	metrics.SetConnectedClusters(clusters)
	log.Infof("Online clusters: %s", strings.Join(clusters, ", "))
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
	s.dispatchStreamsLock.RLock()
	_, clusterAlreadyConnected := s.dispatchStreams[opts.Cluster]
	s.dispatchStreamsLock.RUnlock()
	if clusterAlreadyConnected {
		log.Warnf("Rejected connection from cluster '%s': already connected", opts.Cluster)
		return fmt.Errorf("cluster already connected: %s", opts.Cluster)
	}

	s.dispatchStreamsLock.Lock()
	s.dispatchStreams[opts.Cluster] = stream
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	s.dispatchStreamsLock.Unlock()
	s.reportOnlineClusters()

	// invalidate older deployments
	err := s.invalidateHistoric(stream.Context(), opts.GetCluster(), opts.GetStartupTime().AsTime())
	if err != nil {
		return status.Errorf(codes.Unavailable, err.Error())
	}

	// wait for disconnect
	<-stream.Context().Done()

	s.dispatchStreamsLock.Lock()
	delete(s.dispatchStreams, opts.Cluster)
	log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
	s.dispatchStreamsLock.Unlock()
	s.reportOnlineClusters()

	return nil
}

func (s *dispatchServer) ReportStatus(ctx context.Context, status *pb.DeploymentStatus) (*pb.ReportStatusOpts, error) {
	return &pb.ReportStatusOpts{}, s.HandleDeploymentStatus(ctx, status)
}

// Send all status updates belonging to a specific request
func (s *dispatchServer) StreamStatus(ctx context.Context, channel chan<- *pb.DeploymentStatus) {
	s.statusStreamsLock.Lock()
	s.statusStreams[ctx] = channel
	s.statusStreamsLock.Unlock()

	<-ctx.Done()

	s.statusStreamsLock.Lock()
	delete(s.statusStreams, ctx)
	s.statusStreamsLock.Unlock()

	close(channel)
}
