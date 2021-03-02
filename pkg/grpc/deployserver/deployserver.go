package deployserver

import (
	"context"

	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/grpc/dispatchserver"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/k8sutils"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

	logger.Tracef("Writing deployment to database")

	// Identify resources
	identifiers := k8sutils.Identifiers(resources)
	for i := range identifiers {
		logger.Infof("Resource %d: %s", i+1, identifiers[i])
	}

	cluster := request.GetCluster()
	deployment := database.Deployment{
		ID:      request.GetID(),
		Team:    request.GetTeam(),
		Cluster: &cluster,
		Created: pb.TimestampAsTime(request.GetTime()),
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
				return status.Errorf(codes.Unavailable, "database is unavailable; try again later")
			}
		}
	}

	logger.Tracef("Deployment committed to database")

	return nil
}

func (ds *deployServer) Deploy(ctx context.Context, request *pb.DeploymentRequest) (*pb.DeploymentStatus, error) {
	uuidstr, err := ds.uuidgen()
	if err != nil {
		return nil, err
	}
	request.ID = uuidstr

	err = ds.addToDatabase(ctx, request)
	if err != nil {
		return nil, err
	}

	err = ds.dispatchServer.SendDeploymentRequest(ctx, request)
	if err != nil {
		return nil, err
	}

	st := pb.NewQueuedStatus(request)
	err = ds.dispatchServer.HandleDeploymentStatus(ctx, st)
	if err != nil {
		log.WithFields(request.LogFields()).Errorf("unable to store deployment status in database: %s", err)
	}

	return st, nil
}

func (ds *deployServer) Status(request *pb.DeploymentRequest, server pb.Deploy_StatusServer) error {
	ch := make(chan *pb.DeploymentStatus, 16)

	// Listen for status updates until context is closed
	go ds.dispatchServer.StreamStatus(server.Context(), ch)

	for st := range ch {
		if st.GetRequest().GetID() != request.GetID() {
			continue
		}
		err := server.Send(st)
		if err != nil {
			return err
		}
	}
	return nil
}
