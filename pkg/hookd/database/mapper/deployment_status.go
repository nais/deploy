package database_mapper

import (
	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/pb"
)

func DeploymentStatus(status *pb.DeploymentStatus) database.DeploymentStatus {
	return database.DeploymentStatus{
		ID:           uuid.New().String(),
		DeploymentID: status.GetRequest().GetID(),
		Status:       status.GetState().String(),
		Message:      status.GetMessage(),
		Created:      status.Timestamp(),
	}
}

func PbStatus(status database.DeploymentStatus) *pb.DeploymentStatus {
	return &pb.DeploymentStatus{
		Request: &pb.DeploymentRequest{
			ID: status.DeploymentID,
		},
		Time:    pb.TimeAsTimestamp(status.Created),
		State:   pb.DeploymentState(pb.DeploymentState_value[status.Status]),
		Message: status.Message,
	}
}

func PbRequest(deploy database.Deployment) *pb.DeploymentRequest {
	var cluster string
	if deploy.Cluster != nil {
		cluster = *deploy.Cluster
	}
	return &pb.DeploymentRequest{
		ID:      deploy.ID,
		Time:    pb.TimeAsTimestamp(deploy.Created),
		Cluster: cluster,
		Team:    deploy.Team,
	}
}
