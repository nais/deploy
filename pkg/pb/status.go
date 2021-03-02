package pb

import (
	"fmt"
	"time"
)

func NewErrorStatus(req *DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: fmt.Sprintf("Error: %s", err),
		State:   DeploymentState_error,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewFailureStatus(req *DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: fmt.Sprintf("Deployment failed: %s", err),
		State:   DeploymentState_error,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewInProgressStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: "Resources have been applied to Kubernetes; waiting for new pods to report healthy status",
		State:   DeploymentState_in_progress,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewQueuedStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		State:   DeploymentState_queued,
		Message: "deployment request has been put on the queue for further processing",
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewSuccessStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: fmt.Sprintf("All resources are applied to Kubernetes and reports healthy status."),
		State:   DeploymentState_success,
		Time:    TimeAsTimestamp(time.Now()),
	}
}
