package deployd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	"github.com/navikt/deployment/deployd/pkg/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrNotMyCluster     = fmt.Errorf("your message belongs in another cluster")
	ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")

	deploymentTimeout = time.Second * 300
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

func monitorableResource(resource *unstructured.Unstructured) bool {
	gvk := resource.GroupVersionKind()
	if gvk.Kind == "Application" && gvk.Group == "nais.io" {
		return true
	}
	if gvk.Kind == "Deployment" && (gvk.Group == "apps" || gvk.Group == "extensions") {
		return true
	}
	return false
}

func jsonToResources(json []json.RawMessage) ([]unstructured.Unstructured, error) {
	resources := make([]unstructured.Unstructured, len(json))
	for i := range resources {
		err := resources[i].UnmarshalJSON(json[i])
		if err != nil {
			return nil, fmt.Errorf("resource %d: decoding payload: %s", i+1, err)
		}
	}
	return resources, nil
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

func Run(logger *log.Entry, msg []byte, key, cluster string, kube kubeclient.TeamClientProvider, deployStatus chan *deployment.DeploymentStatus) {
	// Check the validity and authenticity of the message.
	req, err := Prepare(msg, key, cluster)
	if req != nil {
		nl := logger.WithFields(req.LogFields())
		logger.Data = nl.Data // propagate changes down to caller
	}

	if err != nil {
		logger.Tracef("Discarding incoming message: %s", err)
		if err != ErrNotMyCluster {
			deployStatus <- deployment.NewFailureStatus(*req, err)
		}
		return
	}

	p := req.GetPayloadSpec()
	logger.Data["team"] = p.Team

	teamClient, err := kube.TeamClient(p.Team)
	if err != nil {
		deployStatus <- deployment.NewErrorStatus(*req, err)
		return
	}

	rawResources, err := p.JSONResources()
	if err != nil {
		deployStatus <- deployment.NewErrorStatus(*req, fmt.Errorf("unserializing kubernetes resources: %s", err))
		return
	}

	if len(rawResources) == 0 {
		deployStatus <- deployment.NewErrorStatus(*req, fmt.Errorf("no resources to deploy"))
		return
	}

	resources, err := jsonToResources(rawResources)
	if err != nil {
		deployStatus <- deployment.NewErrorStatus(*req, err)
		return
	}

	logger.Infof("Accepting incoming deployment request")

	monitorable := 0

	for index, resource := range resources {
		if monitorableResource(&resource) {

			monitorable += 1
			ns := resource.GetNamespace()
			n := resource.GetName()
			logger.Infof("Monitoring rollout status of deployment '%s' in namespace '%s' for %s", n, ns, deploymentTimeout.String())

			go func() {
				err := teamClient.WaitForDeployment(ns, n, time.Now().Add(deploymentTimeout))
				if err == nil {
					deployStatus <- deployment.NewSuccessStatus(*req)
				} else {
					deployStatus <- deployment.NewFailureStatus(*req, err)
				}
			}()
		}

		deployed, err := teamClient.DeployUnstructured(resource)
		if err != nil {
			deployStatus <- deployment.NewFailureStatus(*req, fmt.Errorf("resource %d: %s", index+1, err))
			return
		}

		metrics.KubernetesResources.Inc()

		logger.Infof("Resource %d: successfully deployed %s", index+1, deployed.GetSelfLink())
	}

	if monitorable > 0 {
		deployStatus <- deployment.NewInProgressStatus(*req)
	} else {
		deployStatus <- deployment.NewSuccessStatus(*req)
	}
}
