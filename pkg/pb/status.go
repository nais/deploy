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
	case DeploymentState_inactive:
	default:
		return false
	}
	return true
}

func (x DeploymentState) IsError() bool {
	switch x {
	case DeploymentState_error:
	case DeploymentState_failure:
	case DeploymentState_inactive:
	default:
		return false
	}
	return true
}

func NewErrorStatus(req *DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: err.Error(),
		State:   DeploymentState_error,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewFailureStatus(req *DeploymentRequest, err error) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: err.Error(),
		State:   DeploymentState_failure,
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

func NewInactiveStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: "Deployment has been lost.",
		State:   DeploymentState_inactive,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewQueuedStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: "Deployment request has been put on the queue for further processing.",
		State:   DeploymentState_queued,
		Time:    TimeAsTimestamp(time.Now()),
	}
}

func NewSuccessStatus(req *DeploymentRequest) *DeploymentStatus {
	return &DeploymentStatus{
		Request: req,
		Message: "Deployment completed successfully.",
		State:   DeploymentState_success,
		Time:    TimeAsTimestamp(time.Now()),
	}
}
