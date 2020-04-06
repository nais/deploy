package deployment

import (
	"fmt"
	"time"
)

func NewErrorStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("Error: %s", err),
		State:       GithubDeploymentState_error,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Time:        TimeAsTimestamp(time.Now()),
	}
}

func NewFailureStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("Deployment failed: %s", err),
		State:       GithubDeploymentState_failure,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Time:        TimeAsTimestamp(time.Now()),
	}
}

func NewInProgressStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: "Resources have been applied to Kubernetes; waiting for new pods to report healthy status",
		State:       GithubDeploymentState_in_progress,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Time:        TimeAsTimestamp(time.Now()),
	}
}

func NewQueuedStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.GetDeployment(),
		DeliveryID:  req.GetDeliveryID(),
		State:       GithubDeploymentState_queued,
		Description: "deployment request has been put on the queue for further processing",
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Time:        TimeAsTimestamp(time.Now()),
	}
}

func NewSuccessStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("All resources are applied to Kubernetes and reports healthy status."),
		State:       GithubDeploymentState_success,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Time:        TimeAsTimestamp(time.Now()),
	}
}
