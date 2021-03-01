package pb

import (
	log "github.com/sirupsen/logrus"
)

const (
	LogFieldDeliveryID           = "delivery_id"
	LogFieldCorrelationID        = "correlation_id"
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
		LogFieldCorrelationID:        m.GetID(),
		LogFieldDeploymentStatusType: m.GetState().String(),
	}
}

func (m *DeploymentRequest) LogFields() log.Fields {
	return log.Fields{
		LogFieldCorrelationID: m.GetID(),
		LogFieldTeam:          m.GetTeam(),
		LogFieldCluster:       m.GetCluster(),
		LogFieldRepository:    m.GetRepository().FullName(),
	}
}
