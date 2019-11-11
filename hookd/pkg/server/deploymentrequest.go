package server

import (
	"encoding/json"
	"fmt"
	"time"

	gh "github.com/google/go-github/v27/github"
	types "github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/github"
)

var (
	// Deployment request's time to live before it is considered too old.
	ttl = time.Minute * 1

	// Payload API version
	payloadVersion = []int32{1, 0, 0}
)

// DeploymentRequestMessage creates a deployment request from user input provided to the deployment API.
func DeploymentRequestMessage(r *DeploymentRequest, deployment *gh.Deployment, deliveryID string) (*types.DeploymentRequest, error) {
	kube, err := types.KubernetesFromJSONResources(r.Resources)
	if err != nil {
		return nil, err
	}
	now := time.Unix(r.Timestamp, 0)
	return &types.DeploymentRequest{
		Deployment: &types.DeploymentSpec{
			Repository: &types.GithubRepository{
				Name:  r.Repository,
				Owner: r.Owner,
			},
			DeploymentID: deployment.GetID(),
		},
		PayloadSpec: &types.Payload{
			Team:       r.Team,
			Version:    payloadVersion,
			Kubernetes: kube,
		},
		DeliveryID: deliveryID,
		Cluster:    r.Cluster,
		Timestamp:  now.Unix(),
		Deadline:   now.Add(ttl).Unix(),
	}, nil
}

// DeploymentRequestFromEvent creates a deployment request from a Github Deployment Event.
// The event is validated, and if any fields are missing, an error is returned.
// Any error from this function should be considered user error.
func DeploymentRequestFromEvent(ev *gh.DeploymentEvent, deliveryID string) (*types.DeploymentRequest, error) {
	repo := ev.GetRepo()
	if repo == nil {
		return nil, fmt.Errorf("no repository specified")
	}

	owner, name, err := github.SplitFullname(repo.GetFullName())
	if err != nil {
		return nil, err
	}

	deployment := ev.GetDeployment()
	if deployment == nil {
		return nil, fmt.Errorf("deployment object is empty")
	}

	cluster := deployment.GetEnvironment()
	if len(cluster) == 0 {
		return nil, fmt.Errorf("environment is not specified")
	}

	payload, err := types.PayloadFromJSON(deployment.Payload)
	err = json.Unmarshal(deployment.Payload, payload)
	if err != nil {
		return nil, fmt.Errorf("payload is invalid: %s", err)
	}

	now := time.Now()
	return &types.DeploymentRequest{
		Deployment: &types.DeploymentSpec{
			Repository: &types.GithubRepository{
				Name:  name,
				Owner: owner,
			},
			DeploymentID: deployment.GetID(),
		},
		PayloadSpec: payload,
		DeliveryID:  deliveryID,
		Cluster:     cluster,
		Timestamp:   now.Unix(),
		Deadline:    now.Add(ttl).Unix(),
	}, nil
}
