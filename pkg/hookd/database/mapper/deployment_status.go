package database_mapper

import (
	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/hookd/database"
	"github.com/navikt/deployment/pkg/pb"
)

func DeploymentStatus(status *pb.DeploymentStatus) database.DeploymentStatus {
	return database.DeploymentStatus{
		ID:           uuid.New().String(),
		DeploymentID: status.GetID(),
		Status:       status.GetState().String(),
		Message:      status.GetMessage(),
		Created:      status.Timestamp(),
	}
}
