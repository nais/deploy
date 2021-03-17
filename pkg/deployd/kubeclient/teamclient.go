package kubeclient

import (
	"fmt"

	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/deployd/strategy"
	"github.com/nais/deploy/pkg/pb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

const (
	CorrelationIDAnnotation = "nais.io/deploymentCorrelationID"
)

type teamClient struct {
	structuredClient   kubernetes.Interface
	unstructuredClient dynamic.Interface
}

type TeamClient interface {
	DeployUnstructured(resource unstructured.Unstructured) (*unstructured.Unstructured, error)
	WaitForDeployment(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus
}

// Implement TeamClient interface
var _ TeamClient = &teamClient{}

func (c *teamClient) gvr(resource *unstructured.Unstructured) (*schema.GroupVersionResource, error) {
	groupResources, err := restmapper.GetAPIGroupResources(c.structuredClient.Discovery())
	if err != nil {
		return nil, fmt.Errorf("unable to run kubernetes resource discovery: %s", err)
	}
	restMapper := restmapper.NewDiscoveryRESTMapper(groupResources)

	gvk := resource.GroupVersionKind()
	gk := schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
	mapping, err := restMapper.RESTMapping(gk, gvk.Version)
	if err != nil {
		return nil, fmt.Errorf("unable to discover resource using REST mapper: %s", err)
	}

	return &mapping.Resource, nil
}

// DeployUnstructured takes a generic unstructured object, discovers its location
// using the Kubernetes API REST mapper, and deploys it to the cluster.
func (c *teamClient) DeployUnstructured(resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	gvr, err := c.gvr(&resource)
	if err != nil {
		return nil, err
	}

	resourceInterface := c.unstructuredClient.Resource(*gvr)
	ns := resource.GetNamespace()

	if len(ns) == 0 {
		return strategy.NewDeployStrategy(resource.GroupVersionKind(), resourceInterface).Deploy(resource)
	}

	return strategy.NewDeployStrategy(resource.GroupVersionKind(), resourceInterface.Namespace(ns)).Deploy(resource)
}

// Returns nil after the next generation of the deployment is successfully rolled out,
// or error if it has not succeeded within the specified deadline.
func (c *teamClient) WaitForDeployment(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	gvk := resource.GroupVersionKind()
	strat := strategy.NewWatchStrategy(gvk, c.structuredClient, c.unstructuredClient)
	return strat.Watch(op, resource)
}
