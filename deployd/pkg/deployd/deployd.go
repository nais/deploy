package deployd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Payload struct {
	Version    [3]int
	Team       string
	Kubernetes struct {
		Resources []json.RawMessage
	}
}

var (
	ErrNotMyCluster     = fmt.Errorf("your message belongs in another cluster")
	ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")
)

func matchesCluster(req deployment.DeploymentRequest, cluster string) error {
	if req.GetCluster() != cluster {
		return ErrNotMyCluster
	}
	return nil
}

func meetsDeadline(req deployment.DeploymentRequest) error {
	deadline := time.Unix(req.GetDeadline(), 0)
	late := time.Since(deadline)
	if late > 0 {
		return ErrDeadlineExceeded
	}
	return nil
}

func deployKubernetes(teamClient kubeclient.TeamClient, logger *log.Entry, payload Payload) error {
	numResources := len(payload.Kubernetes.Resources)
	if numResources == 0 {
		return fmt.Errorf("no resources to deploy")
	}

	for index, r := range payload.Kubernetes.Resources {

		deployed, err := deployJSON(teamClient, r)

		if err != nil {
			return fmt.Errorf("resource %d: %s", index+1, err)
		}

		logger.Infof("resource %d: successfully deployed %s", index+1, deployed.GetSelfLink())
	}

	return nil
}

func deployJSON(teamClient kubeclient.TeamClient, data []byte) (*unstructured.Unstructured, error) {
	resource := unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		return nil, fmt.Errorf("while decoding payload: %s", err)
	}

	return teamClient.DeployUnstructured(resource)
}

// Prepare decodes a string of bytes into a deployment request,
// and decides whether or not to allow a deployment.
//
// If everything is okay, returns a deployment request. Otherwise, an error.
func Prepare(msg []byte, key, cluster string) (*deployment.DeploymentRequest, error) {
	req := &deployment.DeploymentRequest{}

	if err := deployment.UnwrapMessage(msg, key, req); err != nil {
		return nil, err
	}

	// Check if we are the authoritative handler for this message
	if err := matchesCluster(*req, cluster); err != nil {
		return req, err
	}

	// Messages that are too old are discarded
	if err := meetsDeadline(*req); err != nil {
		return req, err
	}

	return req, nil
}

func Run(logger *log.Entry, msg []byte, key, cluster string, kube kubeclient.TeamClientProvider) *deployment.DeploymentStatus {
	// Check the validity and authenticity of the message.
	req, err := Prepare(msg, key, cluster)
	if req != nil {
		repo := req.GetDeployment().GetRepository()
		logger.Data["delivery_id"] = req.GetDeliveryID()
		logger.Data["repository"] = fmt.Sprintf("%s/%s", repo.Owner, repo.Name)
	}

	if err != nil {
		logger.Tracef("discarding incoming message: %s", err)
		if err != ErrNotMyCluster {
			return deployment.NewFailureStatus(*req, err)
		}
		return nil
	}

	payload := Payload{}
	err = json.Unmarshal(req.Payload, &payload)
	if err != nil {
		return deployment.NewFailureStatus(*req, fmt.Errorf("error in payload: %s", err))
	}

	if len(payload.Team) == 0 {
		return deployment.NewFailureStatus(*req, fmt.Errorf("team not specified in deployment payload"))
	}

	logger.Data["team"] = payload.Team

	teamClient, err := kube.TeamClient(payload.Team)
	if err != nil {
		return deployment.NewFailureStatus(*req, err)
	}

	logger.Infof("accepting incoming deployment request")

	if err := deployKubernetes(teamClient, logger, payload); err != nil {
		return deployment.NewFailureStatus(*req, err)
	}

	return deployment.NewSuccessStatus(*req)
}
