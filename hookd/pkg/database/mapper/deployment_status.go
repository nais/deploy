package database_mapper

import (
	"github.com/google/uuid"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/database"
)

func DeploymentStatus(status deployment.DeploymentStatus) database.DeploymentStatus {
	return database.DeploymentStatus{
		ID:           uuid.New().String(),
		DeploymentID: status.GetDeliveryID(),
		Status:       status.GetState().String(),
		Message:      status.GetDescription(),
		Created:      status.Timestamp(),
	}
}
