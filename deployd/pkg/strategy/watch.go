package strategy

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

var (
	requestInterval      = time.Second * 5
	ErrDeploymentTimeout = fmt.Errorf("timeout while waiting for deployment to succeed")
)

type WatchStrategy interface {
	Watch(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error
}

type NoOp struct {
}

func (c NoOp) Watch(logger *log.Entry, resource unstructured.Unstructured, _ time.Time) error {
	logger.Infof("Watch not implemented for resource %s/%s", resource.GroupVersionKind().String(), resource.GetName())
	return nil
}
func NewWatchStrategy(gvk schema.GroupVersionKind, structuredClient kubernetes.Interface, unstructuredClient dynamic.Interface) WatchStrategy {
	if gvk.Group == "nais.io" && gvk.Kind == "Application" {
		return application{unstructuredClient: unstructuredClient, structuredClient: structuredClient}
	}

	if gvk.Kind == "Deployment" && (gvk.Group == "apps" || gvk.Group == "extensions") {
		return deployment{client: structuredClient}
	}

	if gvk.Group == "batch" && gvk.Kind == "Job" && gvk.Version == "v1" {
		return job{client: structuredClient}
	}

	return NoOp{}
}
