package strategy

import (
	"fmt"
	"time"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	requestInterval      = time.Second * 5
	ErrDeploymentTimeout = fmt.Errorf("timeout while waiting for deployment to succeed")
)

type WatchStrategy interface {
	Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus
}

type NoOp struct {
}

func (c NoOp) Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	op.Logger.Debugf("Watch not implemented for resource %s/%s", resource.GroupVersionKind().String(), resource.GetName())
	return nil
}

func NewWatchStrategy(gvk schema.GroupVersionKind, client kubeclient.Interface) WatchStrategy {
	if gvk.Group == "nais.io" && gvk.Kind == "Application" {
		return application{client: client}
	}

	if gvk.Kind == "Deployment" && (gvk.Group == "apps" || gvk.Group == "extensions") {
		return deployment{client: client}
	}

	if gvk.Group == "batch" && gvk.Kind == "Job" && gvk.Version == "v1" {
		return job{client: client}
	}

	return NoOp{}
}
