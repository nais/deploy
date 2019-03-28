package deployment

import (
	"fmt"
)

func NewFailureStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("deployment failed: %s", err),
		State:       GithubDeploymentState_failure,
		DeliveryID:  req.GetDeliveryID(),
	}
}

func NewSuccessStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Deployment:  req.Deployment,
		Description: fmt.Sprintf("deployment succeeded"),
		State:       GithubDeploymentState_success,
		DeliveryID:  req.GetDeliveryID(),
	}
}
