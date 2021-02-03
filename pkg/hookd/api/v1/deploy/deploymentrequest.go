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
		Deployment: &types.DeploymentSpec{
			Repository: &types.GithubRepository{
				Name:  r.Repository,
				Owner: r.Owner,
			},
			Environment: r.Environment,
			Ref:         r.Ref,
		},
		PayloadSpec: &types.Payload{
			Team:       r.Team,
			Version:    payloadVersion,
			Kubernetes: kube,
		},
		DeliveryID: deliveryID,
		Cluster:    r.Cluster,
		Time:       types.TimeAsTimestamp(now),
		Deadline:   now.Add(ttl).Unix(),
	}, nil
}
