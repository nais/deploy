// package deployServer provides message streams between hookd and deployd

package dispatchserver

import (
	"context"
	"fmt"

	"github.com/nais/api/pkg/apiclient/protoapi"
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
	conn, online := s.onlineClustersMap[request.Cluster]
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

	clearSpan := func() {
		span.End()
		s.traceSpansLock.Lock()
		delete(s.traceSpans, request.ID)
		s.traceSpansLock.Unlock()
	}

	wait := make(chan error, 1)
	// Guard against the receiving Deployments goroutine having gone away (e.g.
	// because it was displaced by a newer connection), which would otherwise
	// block this send forever.
	select {
	case conn.requests <- &requestWithWait{request: request, wait: wait}:
	case <-conn.quit:
		clearSpan()
		return status.Errorf(codes.Unavailable, "cluster '%s' connection was closed", request.Cluster)
	case <-ctx.Done():
		clearSpan()
		return status.Errorf(codes.Unavailable, "send deployment request: %s", ctx.Err())
	}

	if err := <-wait; err != nil {
		clearSpan()
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
			return status.Error(codes.FailedPrecondition, err.Error())
		}
		return status.Errorf(codes.Unavailable, "write deployment status to database: %s", err)
	}

	metrics.UpdateQueue(st)
	logger := log.WithFields(st.LogFields())
	logger.Debugf("Saved deployment status in database")

	err = s.writeDeploymentStatusToNaisApi(ctx, st)
	if err != nil {
		logger.WithError(err).Errorf("Write deployment status to Nais API")
	}

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

func (s *dispatchServer) writeDeploymentStatusToNaisApi(ctx context.Context, status *pb.DeploymentStatus) error {
	reqID := status.GetRequest().GetID()
	msg := status.GetMessage()
	_, err := s.apiClient.CreateDeploymentStatus(ctx, protoapi.CreateDeploymentStatusRequest_builder{
		ExternalDeploymentId: &reqID,
		CreatedAt:            status.GetTime(),
		State:                convertDeploymentState(status.GetState()),
		Message:              &msg,
	}.Build())
	return err
}

func convertDeploymentState(deploymentState pb.DeploymentState) *protoapi.DeploymentState {
	ret := protoapi.DeploymentState_pending
	switch deploymentState {
	case pb.DeploymentState_success:
		ret = protoapi.DeploymentState_success
	case pb.DeploymentState_error:
		ret = protoapi.DeploymentState_error
	case pb.DeploymentState_failure:
		ret = protoapi.DeploymentState_failure
	case pb.DeploymentState_inactive:
		ret = protoapi.DeploymentState_inactive
	case pb.DeploymentState_in_progress:
		ret = protoapi.DeploymentState_in_progress
	case pb.DeploymentState_queued:
		ret = protoapi.DeploymentState_queued
	}
	return &ret
}
