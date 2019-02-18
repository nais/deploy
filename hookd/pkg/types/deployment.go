package types

import (
	"time"
)

var ProtocolVersion = [3]int{1, 0, 0}

type Deployment struct {
	ProtocolVersion [3]int
	Timestamp       time.Time
	CorrelationID   string
	RepositoryOwner string
	RepositoryName  string
	DeploymentID    int64
	Cluster         string
}

type DeploymentRequest struct {
	Deployment
	Deadline time.Time
	Payload  string
}

type DeploymentResponse struct {
	Deployment
	State       string
	Description string
}
