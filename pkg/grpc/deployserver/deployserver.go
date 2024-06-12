package deployserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/nais/deploy/pkg/grpc/dispatchserver"
	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/k8sutils"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var ErrDatabaseUnavailable = status.Errorf(codes.Unavailable, "database is unavailable; try again later")

type deployServer struct {
	pb.UnimplementedDeployServer
	dispatchServer  dispatchserver.DispatchServer
	deploymentStore database.DeploymentStore
}

func New(dispatchServer dispatchserver.DispatchServer, deploymentStore database.DeploymentStore) pb.DeployServer {
	return &deployServer{
		deploymentStore: deploymentStore,
		dispatchServer:  dispatchServer,
	}
}

func (ds *deployServer) uuidgen() (string, error) {
	uuidstr, err := uuid.NewRandom()
	if err != nil {
		return "", status.Errorf(codes.Unavailable, err.Error())
	}
	return uuidstr.String(), nil
}

func (ds *deployServer) addToDatabase(ctx context.Context, request *pb.DeploymentRequest) error {
	logger := log.WithFields(request.LogFields())

	resources, err := k8sutils.ResourcesFromDeploymentRequest(request)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "invalid Kubernetes resources in request: %s", err)
	}

	// Identify resources
	identifiers := k8sutils.Identifiers(resources)
	for i := range identifiers {
		logger.Infof("Resource %d: %s", i+1, identifiers[i])
	}

	cluster := request.GetCluster()
	deployment := database.Deployment{
		ID:               request.GetID(),
		Team:             request.GetTeam(),
		Cluster:          &cluster,
		Created:          pb.TimestampAsTime(request.GetTime()),
		GitHubRepository: request.GetRepository().FullNamePtr(),
	}

	// Write deployment request to database
	err = ds.deploymentStore.WriteDeployment(ctx, deployment)

	if err == nil {
		// Write metadata of Kubernetes resources to database
		for i, id := range identifiers {
			uuidstr, err := ds.uuidgen()
			if err != nil {
				return err
			}

			err = ds.deploymentStore.WriteDeploymentResource(ctx, database.DeploymentResource{
				ID:           uuidstr,
				DeploymentID: deployment.ID,
				Index:        i,
				Group:        id.Group,
				Version:      id.Version,
				Kind:         id.Kind,
				Name:         id.Name,
				Namespace:    id.Namespace,
			})

			if err != nil {
				logger.Error(err)
				return ErrDatabaseUnavailable
			}
		}
	} else {
		logger.Error(err)
		return ErrDatabaseUnavailable
	}

	return nil
}

func (ds *deployServer) Deploy(ctx context.Context, request *pb.DeploymentRequest) (*pb.DeploymentStatus, error) {
	uuidstr, err := ds.uuidgen()
	if err != nil {
		return nil, err
	}
	request.ID = uuidstr

	logger := log.WithFields(request.LogFields())
	logger.Infof("Received deployment request")

	logger.Debugf("Writing deployment to database")
	err = ds.addToDatabase(ctx, request)
	if err != nil {
		logger.Errorf("Write deployment to database: %s", err)
		return nil, err
	}
	logger.Debugf("Deployment committed to database")

	err = ds.dispatchServer.SendDeploymentRequest(ctx, request)
	if err != nil {
		logger.Errorf("Dispatch deployment: %s", err)
		return nil, err
	}

	st := pb.NewQueuedStatus(request)
	err = ds.dispatchServer.HandleDeploymentStatus(ctx, st)
	if err != nil {
		logger.Errorf("Unable to store deployment status in database: %s", err)
	}

	return st, nil
}

func (ds *deployServer) Status(request *pb.DeploymentRequest, server pb.Deploy_StatusServer) error {
	logger := log.WithFields(request.LogFields())
	logger.Debugf("Status stream opened")
	defer logger.Debugf("Status stream closed")

	dbStatus, err := ds.deploymentStore.DeploymentStatus(server.Context(), request.GetID())
	if err == nil && len(dbStatus) > 0 {
		err = server.Send(database_mapper.PbStatus(dbStatus[0]))
	}
	if err != nil {
		return err
	}

	ch := make(chan *pb.DeploymentStatus, 16)

	// Listen for status updates until context is closed
	go ds.dispatchServer.StreamStatus(server.Context(), ch)

	for st := range ch {
		if st.GetRequest().GetID() != request.GetID() {
			continue
		}
		err := server.Send(st)
		if err != nil {
			logger := log.WithFields(st.LogFields())
			logger.WithError(err).Error("send status to client")
			return err
		}
	}
	return nil
}
