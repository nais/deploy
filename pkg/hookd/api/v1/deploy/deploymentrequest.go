package api_v1_deploy

import (
	"time"

	types "github.com/navikt/deployment/pkg/pb"
)

var (
	// Deployment request's time to live before it is considered too old.
	ttl = time.Minute * 1

	// Payload API version
	payloadVersion = []int32{1, 0, 0}
)

// DeploymentRequestMessage creates a deployment request from user input provided to the deployment API.
func DeploymentRequestMessage(r *DeploymentRequest, deliveryID string) (*types.DeploymentRequest, error) {
	kube, err := types.KubernetesFromJSONResources(r.Resources)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &types.DeploymentRequest{
		ID:                deliveryID,
		Time:              types.TimeAsTimestamp(now),
		Deadline:          types.TimeAsTimestamp(now.Add(ttl)),
		Cluster:           r.Cluster,
		Team:              r.Team,
		GitRefSha:         r.Ref,
		Kubernetes:        kube,
		Repository:        &types.GithubRepository{
			Owner: r.Owner,
			Name:  r.Repository,
		},
		GithubEnvironment: r.Environment,
	}, nil
}