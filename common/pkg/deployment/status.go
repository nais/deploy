package deployment

import (
	"fmt"
)

func NewErrorStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("Error in deployment request: %s", err),
		State:       GithubDeploymentState_error,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Timestamp:   req.GetTimestamp(),
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
		Timestamp:   req.GetTimestamp(),
	}
}

func NewInProgressStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("Resources have been applied to Kubernetes; waiting for new pods to report healthy status."),
		State:       GithubDeploymentState_in_progress,
		DeliveryID:  req.GetDeliveryID(),
		Team:        req.GetPayloadSpec().GetTeam(),
		Cluster:     req.GetCluster(),
		Timestamp:   req.GetTimestamp(),
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
		Timestamp:   req.GetTimestamp(),
	}
}
