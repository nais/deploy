// package deployServer provides message streams between hookd and deployd

package dispatchserver

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/hookd/metrics"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	requestTimeout  = time.Second * 5
	errNoRepository = fmt.Errorf("no repository specified")
)

func (s *dispatchServer) SendDeploymentRequest(ctx context.Context, request *pb.DeploymentRequest) error {
	s.dispatchStreamsLock.RLock()
	stream, online := s.dispatchStreams[request.Cluster]
	s.dispatchStreamsLock.RUnlock()
	if !online {
		return status.Errorf(codes.Unavailable, "cluster '%s' is offline", request.Cluster)
	}

	err := stream.Send(request)
	if err != nil {
		return err
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
		logger.Infof("Deployment finished")
	}

	return nil
}
