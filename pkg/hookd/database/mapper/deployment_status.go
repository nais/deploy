package database_mapper

import (
	"github.com/google/uuid"
	"github.com/navikt/deployment/pkg/pb"
	"github.com/navikt/deployment/pkg/hookd/database"
)

func DeploymentStatus(status pb.DeploymentStatus) database.DeploymentStatus {
	return database.DeploymentStatus{
		ID:           uuid.New().String(),
		DeploymentID: status.GetDeliveryID(),
		Status:       status.GetState().String(),
		Message:      status.GetDescription(),
		Created:      status.Timestamp(),
	}
}
