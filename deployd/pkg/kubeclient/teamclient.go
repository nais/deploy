package kubeclient

import (
	"fmt"
	"github.com/navikt/deployment/deploy/pkg/deployer/strategy"
	"time"

	log "github.com/sirupsen/logrus"
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
	WaitForDeployment(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error
}

// Implement TeamClient interface
var _ TeamClient = &teamClient{}

// DeployUnstructured takes a generic unstructured object, discovers its location
// using the Kubernetes API REST mapper, and deploys it to the cluster.
func (c *teamClient) DeployUnstructured(resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
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

	clusterResource := c.unstructuredClient.Resource(mapping.Resource)
	ns := resource.GetNamespace()

	if len(ns) == 0 {
		return c.createOrUpdate(clusterResource, resource)
	}

	namespacedResource := clusterResource.Namespace(ns)

	return strategy.NewDeployStrategy(gvk, namespacedResource).Deploy(resource)
}

// Returns nil after the next generation of the deployment is successfully rolled out,
// or error if it has not succeeded within the specified deadline.
func (c *teamClient) WaitForDeployment(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error {
	gvk := resource.GroupVersionKind()
	return strategy.NewWatchStrategy(gvk,c.structuredClient,c.unstructuredClient).Watch(logger,resource,deadline)
}

