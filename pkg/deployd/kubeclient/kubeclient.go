package kubeclient

import (
	"fmt"

	"github.com/nais/deploy/pkg/deployd/teamconfig"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth" // Needed for auth side effect
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
)

// Provide a Kubernetes client that knows how to deal with unstructured resources and team impersonation.
type Interface interface {
	// Return a Kubernetes client.
	Kubernetes() kubernetes.Interface

	// Return an object that knows how to do CRUD on the specified unstructured resource.
	ResourceInterface(resource *unstructured.Unstructured) (dynamic.ResourceInterface, error)

	// Return a new client of the same type, but using the team's credentials
	Impersonate(team string) (Interface, error)
}

type client struct {
	static  kubernetes.Interface
	dynamic dynamic.Interface
	config  *rest.Config
}

var _ Interface = &client{}

func (c *client) Kubernetes() kubernetes.Interface {
	return c.static
}

func (c *client) Impersonate(team string) (Interface, error) {
	config, err := teamconfig.Generate(*c.config, team)
	if err != nil {
		return nil, err
	}
	return New(config)
}

// Given a unstructured Kubernetes resource, return a dynamic client that knows how to apply it to the cluster.
func (c *client) ResourceInterface(resource *unstructured.Unstructured) (dynamic.ResourceInterface, error) {
	gvr, err := c.gvr(resource)
	if err != nil {
		return nil, err
	}

	resourceInterface := c.dynamic.Resource(*gvr)
	ns := resource.GetNamespace()

	if len(ns) == 0 {
		return resourceInterface, nil
	}

	return resourceInterface.Namespace(ns), nil
}

func New(config *rest.Config) (Interface, error) {
	cli, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &client{
		static:  cli,
		dynamic: dyn,
		config:  config,
	}, nil
}

// Given a unstructured Kubernetes resource, return a GroupVersionResource that identifies it in the cluster.
func (c *client) gvr(resource *unstructured.Unstructured) (*schema.GroupVersionResource, error) {
	groupResources, err := restmapper.GetAPIGroupResources(c.static.Discovery())
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
