package pb

import (
	"fmt"
	"time"
)

func NewErrorStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Message: fmt.Sprintf("Error: %s", err),
		State:   DeploymentState_error,
		ID:      req.GetID(),
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewFailureStatus(req DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Message: fmt.Sprintf("Deployment failed: %s", err),
		State:   DeploymentState_error,
		ID:      req.GetID(),
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewInProgressStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Message: "Resources have been applied to Kubernetes; waiting for new pods to report healthy status",
		State:   DeploymentState_in_progress,
		ID:      req.GetID(),
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewQueuedStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		ID:      req.GetID(),
		State:   DeploymentState_queued,
		Message: "deployment request has been put on the queue for further processing",
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewSuccessStatus(req DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Message: fmt.Sprintf("All resources are applied to Kubernetes and reports healthy status."),
		State:   DeploymentState_success,
		ID:      req.GetID(),
		Time:    TimeAsTimestamp(time.Now()),
	}
}
