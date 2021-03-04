package pb

import (
	"fmt"
	"time"
)

func (x DeploymentState) Finished() bool {
	switch x {
	case DeploymentState_success:
	case DeploymentState_error:
	case DeploymentState_failure:
	default:
		return false
	}
	return true
}

func (x DeploymentState) IsError() bool {
	switch x {
	case DeploymentState_error:
	case DeploymentState_failure:
	default:
		return false
	}
	return true
}

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

func NewInProgressStatus(req *DeploymentRequest, format string, args ...interface{}) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: fmt.Sprintf(format, args...),
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
