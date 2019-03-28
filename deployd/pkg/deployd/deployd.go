package deployd

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/restmapper"
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

// deployKubernetes something to Kubernetes
func deployKubernetes(log *log.Entry, req deployment.DeploymentRequest, kube *kubeclient.Client) error {
	payload := Payload{}
	err := json.Unmarshal(req.Payload, &payload)
	if err != nil {
		return fmt.Errorf("while decoding payload: %s", err)
	}

	numResources := len(payload.Kubernetes.Resources)
	if numResources == 0 {
		return fmt.Errorf("no resources to deploy")
	}

	if len(payload.Team) == 0 {
		return fmt.Errorf("team not specified in deployment payload")
	}

	log.Infof("deploying %d resources to Kubernetes on behalf of team %s", numResources, payload.Team)

	kcli, dcli, err := kube.TeamClient(payload.Team)
	if err != nil {
		return err
	}

	groupResources, err := restmapper.GetAPIGroupResources(kcli.Discovery())
	if err != nil {
		return fmt.Errorf("unable to run kubernetes resource discovery: %s", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	for index, r := range payload.Kubernetes.Resources {

		deployed, err := deployJSON(r, restMapper, dcli)

		if err != nil {
			return fmt.Errorf("resource %d: %s", index+1, err)
		}

		log.Infof("resource %d: team %s successfully deployed %s", index+1, payload.Team, deployed.GetSelfLink())
	}

	return nil
}

func deployJSON(data []byte, restMapper meta.RESTMapper, client dynamic.Interface) (*unstructured.Unstructured, error) {

	resource := unstructured.Unstructured{}
	err := resource.UnmarshalJSON(data)
	if err != nil {
		return nil, fmt.Errorf("while decoding payload: %s", err)
	}

	gvk := resource.GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := restMapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to discover resource using REST mapper: %s", err)
	}

	clusterResource := client.Resource(mapping.Resource)
	ns := resource.GetNamespace()
	deployed := &unstructured.Unstructured{}

	if len(ns) > 0 {
		namespacedResource := clusterResource.Namespace(ns)
		deployed, err = namespacedResource.Update(&resource, metav1.UpdateOptions{})
		if errors.IsNotFound(err) {
			deployed, err = namespacedResource.Create(&resource, metav1.CreateOptions{})
		}
	} else {
		deployed, err = clusterResource.Update(&resource, metav1.UpdateOptions{})
		if errors.IsNotFound(err) {
			deployed, err = clusterResource.Create(&resource, metav1.CreateOptions{})
		}
	}

	if err != nil {
		return nil, fmt.Errorf("deploying to Kubernetes: %s", err)
	}

	return deployed, nil
}

// Prepare decodes a string of bytes into a deployment request,
// and decides whether or not to allow a deployment.
//
// If everything is okay, returns a deployment request. Otherwise, an error.
func Prepare(msg []byte, key, cluster string, kube *kubeclient.Client) (*deployment.DeploymentRequest, error) {
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

func Run(logger *log.Entry, msg []byte, key, cluster string, kube *kubeclient.Client) (*deployment.DeploymentStatus, error) {
	// Check the validity and authenticity of the message.
	req, err := Prepare(msg, key, cluster, kube)
	if req != nil {
		repo := req.GetDeployment().GetRepository()
		logger = log.WithFields(log.Fields{
			"delivery_id": req.GetDeliveryID(),
			"repository":  fmt.Sprintf("%s/%s", repo.Owner, repo.Name),
		})
	}

	if err != nil {
		logger.Tracef("discarding incoming message: %s", err)
		if err != ErrNotMyCluster {
			return deployment.NewFailureStatus(*req, err), nil
		}
		return nil, nil
	}

	logger.Infof("accepting incoming deployment request for %s", req.String())

	err = deployKubernetes(logger, *req, kube)

	if err != nil {
		return deployment.NewFailureStatus(*req, err), nil
	}

	return deployment.NewSuccessStatus(*req), nil
}
