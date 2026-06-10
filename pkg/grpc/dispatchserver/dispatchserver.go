package dispatchserver

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/nais/api/pkg/apiclient/protoapi"
	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/hookd/metrics"

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
	onlineClustersLock sync.RWMutex
	onlineClustersMap  map[string]*clusterConnection
	statusStreamsLock  sync.RWMutex
	statusStreams      map[context.Context]chan<- *pb.DeploymentStatus
	traceSpans         map[string]trace.Span
	traceSpansLock     sync.RWMutex
	db                 database.DeploymentStore
	apiClient          protoapi.DeploymentsClient
}

var _ DispatchServer = &dispatchServer{}

type requestWithWait struct {
	request *pb.DeploymentRequest
	wait    chan error
}

// clusterConnection represents a single deployd stream connection for a cluster.
type clusterConnection struct {
	requests chan *requestWithWait
	// startup is the reported startup time of the connected deployd instance.
	// It is used as a stable, monotonic tiebreaker to decide which connection
	// wins when a cluster is connected more than once (see Deployments).
	startup time.Time
	// quit is closed when this connection is displaced by a newer one, asking
	// the owning Deployments goroutine to terminate.
	quit chan struct{}
}

func New(db database.DeploymentStore, apiClient protoapi.DeploymentsClient) DispatchServer {
	server := &dispatchServer{
		onlineClustersMap: make(map[string]*clusterConnection),
		statusStreams:     make(map[context.Context]chan<- *pb.DeploymentStatus),
		traceSpans:        make(map[string]trace.Span),
		db:                db,
		apiClient:         apiClient,
	}

	return server
}

func (s *dispatchServer) onlineClusters() []string {
	s.onlineClustersLock.RLock()
	defer s.onlineClustersLock.RUnlock()

	clusters := make([]string, 0, len(s.onlineClustersMap))
	for k := range s.onlineClustersMap {
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
	conn := &clusterConnection{
		requests: make(chan *requestWithWait),
		startup:  opts.GetStartupTime().AsTime(),
		quit:     make(chan struct{}),
	}

	s.onlineClustersLock.Lock()
	if existing, alreadyConnected := s.onlineClustersMap[opts.Cluster]; alreadyConnected {
		if conn.startup.Before(existing.startup) {
			s.onlineClustersLock.Unlock()
			log.Warnf("Rejected connection from cluster '%s': a newer connection is already established", opts.Cluster)
			return status.Errorf(codes.AlreadyExists, "a newer connection for cluster '%s' is already established", opts.Cluster)
		}
		log.Warnf("Displacing existing connection from cluster '%s' with a newer one", opts.Cluster)
		close(existing.quit)
	}
	s.onlineClustersMap[opts.Cluster] = conn
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	s.onlineClustersLock.Unlock()
	s.reportOnlineClusters()

	defer func() {
		s.onlineClustersLock.Lock()
		if s.onlineClustersMap[opts.Cluster] == conn {
			delete(s.onlineClustersMap, opts.Cluster)
		}
		s.onlineClustersLock.Unlock()
		s.reportOnlineClusters()
	}()

	// invalidate older deployments
	err := s.invalidateHistoric(stream.Context(), opts.GetCluster(), opts.GetStartupTime().AsTime())
	if err != nil {
		return status.Error(codes.Unavailable, err.Error())
	}

	for {
		select {
		case <-stream.Context().Done():
			log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
			return nil
		case <-conn.quit:
			log.Warnf("Connection from cluster '%s' displaced by a newer connection", opts.Cluster)
			return status.Errorf(codes.Aborted, "connection displaced by a newer connection")
		case req := <-conn.requests:
			err := stream.Send(req.request)
			req.wait <- err
			if err != nil {
				return err
			}
		case <-time.After(15 * time.Minute):
			log.Warnf("Connection from cluster '%s' timed out", opts.Cluster)
			return fmt.Errorf("timeout")
		}
	}
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
