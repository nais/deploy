package strategy

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"time"
)

var (
	requestInterval                = time.Second * 5
	ErrDeploymentTimeout           = fmt.Errorf("timeout while waiting for deployment to succeed")
	ErrWatchStrategyNotImplemented = fmt.Errorf("watch for this resource is not implemented@")
)

type WatchStrategy interface {
	Watch(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error
}

type notImplemented struct {
}

func (c notImplemented) Watch(logger *log.Entry, resource unstructured.Unstructured, _ time.Time) error {
	logger.Errorf("Watch not implemented for resource %s/%s", resource.GroupVersionKind().String(), resource.GetName())
	return ErrWatchStrategyNotImplemented
}

func NewWatchStrategy(gvk schema.GroupVersionKind, structuredClient kubernetes.Interface, unstructuredClient dynamic.Interface) WatchStrategy {
	if gvk.Group == "nais.io" && gvk.Kind == "Application" && gvk.Version == "v1alpha1" {
		return application{unstructuredClient: unstructuredClient, structuredClient: structuredClient}
	}

	if gvk.Group == "apps" && gvk.Kind == "Deployment" && gvk.Version == "v1" {
		return deployment{client: structuredClient}
	}

	if gvk.Group == "batch" && gvk.Kind == "Job" && gvk.Version == "v1" {
		return job{client: structuredClient}
	}

	return notImplemented{}

}
