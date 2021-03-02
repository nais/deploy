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
	LogFieldDeploymentStatusType = "deployment_status"
)

func (x *DeploymentStatus) LogFields() log.Fields {
	return log.Fields{
		LogFieldCorrelationID:        x.GetRequest().GetID(),
		LogFieldDeploymentStatusType: x.GetState().String(),
	}
}

func (x *DeploymentRequest) LogFields() log.Fields {
	return log.Fields{
		LogFieldCorrelationID: x.GetID(),
		LogFieldTeam:          x.GetTeam(),
		LogFieldCluster:       x.GetCluster(),
		LogFieldRepository:    x.GetRepository().FullName(),
	}
}
