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
	onlineClustersMap  map[string]chan<- *requestWithWait
	statusStreamsLock  sync.RWMutex
	statusStreams      map[context.Context]chan<- *pb.DeploymentStatus
	traceSpans         map[string]trace.Span
	traceSpansLock     sync.RWMutex
	db                 database.DeploymentStore
}

var _ DispatchServer = &dispatchServer{}

type requestWithWait struct {
	request *pb.DeploymentRequest
	wait    chan error
}

func New(db database.DeploymentStore) DispatchServer {
	server := &dispatchServer{
		onlineClustersMap: make(map[string]chan<- *requestWithWait),
		statusStreams:     make(map[context.Context]chan<- *pb.DeploymentStatus),
		traceSpans:        make(map[string]trace.Span),
		db:                db,
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
	c := make(chan *requestWithWait)
	s.onlineClustersLock.RLock()
	_, clusterAlreadyConnected := s.onlineClustersMap[opts.Cluster]
	s.onlineClustersLock.RUnlock()
	if clusterAlreadyConnected {
		log.Warnf("Rejected connection from cluster '%s': already connected", opts.Cluster)
		return fmt.Errorf("cluster already connected: %s", opts.Cluster)
	}

	s.onlineClustersLock.Lock()
	s.onlineClustersMap[opts.Cluster] = c
	log.Infof("Connection opened from cluster '%s'", opts.Cluster)
	s.onlineClustersLock.Unlock()
	s.reportOnlineClusters()

	defer func() {
		s.onlineClustersLock.Lock()
		delete(s.onlineClustersMap, opts.Cluster)
		s.onlineClustersLock.Unlock()
		s.reportOnlineClusters()
	}()

	// invalidate older deployments
	err := s.invalidateHistoric(stream.Context(), opts.GetCluster(), opts.GetStartupTime().AsTime())
	if err != nil {
		return status.Errorf(codes.Unavailable, err.Error())
	}

	for {
		select {
		case <-stream.Context().Done():
			log.Warnf("Connection from cluster '%s' closed", opts.Cluster)
			return nil
		case req := <-c:
			err := stream.Send(req.request)
			req.wait <- err
		case <-time.After(30 * time.Minute):
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
