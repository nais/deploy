// package deployServer provides message streams between hookd and deployd

package dispatchserver

import (
	"context"
	"fmt"

	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/hookd/metrics"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	log "github.com/sirupsen/logrus"
	otrace "go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (s *dispatchServer) SendDeploymentRequest(ctx context.Context, request *pb.DeploymentRequest) error {
	s.onlineClustersLock.RLock()
	c, online := s.onlineClustersMap[request.Cluster]
	s.onlineClustersLock.RUnlock()
	if !online {
		return status.Errorf(codes.Unavailable, "cluster '%s' is offline", request.Cluster)
	}

	ctx = telemetry.WithTraceParent(ctx, request.TraceParent)
	s.traceSpansLock.Lock()
	ctx, span := telemetry.Tracer().Start(ctx, "Deploy", otrace.WithSpanKind(otrace.SpanKindServer))
	s.traceSpans[request.ID] = span
	request.TraceParent = telemetry.TraceParentHeader(ctx)
	s.traceSpansLock.Unlock()

	wait := make(chan error, 1)
	c <- &requestWithWait{request: request, wait: wait}
	if err := <-wait; err != nil {
		span.End()
		s.traceSpansLock.Lock()
		delete(s.traceSpans, request.ID)
		s.traceSpansLock.Unlock()
		return fmt.Errorf("send deployment request: %w", err)
	}

	log.WithFields(request.LogFields()).Debugf("Deployment request sent to deployd")

	return nil
}

func (s *dispatchServer) HandleDeploymentStatus(ctx context.Context, st *pb.DeploymentStatus) error {
	s.statusStreamsLock.RLock()
	for _, ch := range s.statusStreams {
		ch <- st
	}
	s.statusStreamsLock.RUnlock()

	dbStatus := database_mapper.DeploymentStatus(st)
	err := s.db.WriteDeploymentStatus(ctx, dbStatus)
	if err != nil {
		if database.IsErrForeignKeyViolation(err) {
			return status.Errorf(codes.FailedPrecondition, err.Error())
		}
		return status.Errorf(codes.Unavailable, "write deployment status to database: %s", err)
	}

	metrics.UpdateQueue(st)

	logger := log.WithFields(st.LogFields())
	logger.Debugf("Saved deployment status in database")

	if st.GetState().Finished() {
		deployID := st.GetRequest().GetID()
		s.traceSpansLock.Lock()
		if span, ok := s.traceSpans[deployID]; ok {
			span.End()
			delete(s.traceSpans, deployID)
		}
		s.traceSpansLock.Unlock()
		logger.Infof("Deployment finished")
	}

	return nil
}
