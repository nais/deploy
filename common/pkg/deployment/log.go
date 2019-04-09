package deployment

import (
	log "github.com/sirupsen/logrus"
)

const (
	LogFieldDeliveryID           = "delivery_id"
	LogFieldRepository           = "repository"
	LogFieldDeploymentID         = "deployment_id"
	LogFieldCluster              = "deployment_cluster"
	LogFieldTeam                 = "team"
	LogFieldEventType            = "event_type"
	LogFieldDeploymentStatusID   = "deployment_status_id"
	LogFieldDeploymentStatusType = "deployment_status"
)

func (m *DeploymentStatus) LogFields() log.Fields {
	return log.Fields{
		LogFieldDeliveryID:           m.GetDeliveryID(),
		LogFieldRepository:           m.GetDeployment().GetRepository().FullName(),
		LogFieldDeploymentID:         m.GetDeployment().GetDeploymentID(),
		LogFieldDeploymentStatusType: m.GetState().String(),
	}
}

func (m *DeploymentRequest) LogFields() log.Fields {
	return log.Fields{
		LogFieldDeliveryID:   m.GetDeliveryID(),
		LogFieldDeploymentID: m.GetDeployment().GetDeploymentID(),
		LogFieldTeam:         m.GetPayloadSpec().GetTeam(),
		LogFieldCluster:      m.GetCluster(),
		LogFieldRepository:   m.GetDeployment().GetRepository().FullName(),
	}
}
