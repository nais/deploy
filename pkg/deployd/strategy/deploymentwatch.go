package strategy

import (
	"fmt"
	"strconv"
	"time"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type deployment struct {
	client kubeclient.Interface
}

func (d deployment) Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	var cur *apps.Deployment
	var nova *apps.Deployment
	var err error
	var resourceVersion int
	var updated bool

	client := d.client.Kubernetes().AppsV1().Deployments(resource.GetNamespace())
	deadline, _ := op.Context.Deadline()

	// For native Kubernetes deployment objects, get the current deployment object.
	for deadline.After(time.Now()) {
		cur, err = client.Get(resource.GetName(), metav1.GetOptions{})
		if err == nil {
			resourceVersion, _ = strconv.Atoi(cur.GetResourceVersion())
			op.Logger.Debugf("Found current deployment at version %d: %s", resourceVersion, cur.GetSelfLink())
		} else if errors.IsNotFound(err) {
			op.Logger.Debugf("Deployment '%s' in namespace '%s' is not currently present in the cluster.", resource.GetName(), resource.GetNamespace())
		} else {
			op.Logger.Debugf("Recoverable error while polling for deployment object: %s", err)
			time.Sleep(requestInterval)
			continue
		}
		break
	}

	// Wait until the new deployment object is present in the cluster.
	for deadline.After(time.Now()) {
		nova, err = client.Get(resource.GetName(), metav1.GetOptions{})
		if err != nil {
			time.Sleep(requestInterval)
			continue
		}

		rv, _ := strconv.Atoi(nova.GetResourceVersion())
		if rv > resourceVersion {
			op.Logger.Tracef("New deployment appeared at version %d: %s", rv, cur.GetSelfLink())
			resourceVersion = rv
			updated = true
		}

		if updated && deploymentComplete(nova, &nova.Status) {
			return pb.NewSuccessStatus(op.Request)
		}

		op.Logger.WithFields(log.Fields{
			"deployment_replicas":            nova.Status.Replicas,
			"deployment_updated_replicas":    nova.Status.UpdatedReplicas,
			"deployment_available_replicas":  nova.Status.AvailableReplicas,
			"deployment_observed_generation": nova.Status.ObservedGeneration,
		}).Debugf("Still waiting for deployment to finish rollout...")

		time.Sleep(requestInterval)
	}

	if err != nil {
		return pb.NewErrorStatus(op.Request, fmt.Errorf("%s; last error was: %s", ErrDeploymentTimeout, err))
	}

	return pb.NewErrorStatus(op.Request, ErrDeploymentTimeout)
}

// deploymentComplete considers a deployment to be complete once all of its desired replicas
// are updated and available, and no old pods are running.
//
// Copied verbatim from
// https://github.com/kubernetes/kubernetes/blob/74bcefc8b2bf88a2f5816336999b524cc48cf6c0/pkg/controller/deployment/util/deployment_util.go#L745
func deploymentComplete(deployment *apps.Deployment, newStatus *apps.DeploymentStatus) bool {
	return newStatus.UpdatedReplicas == *(deployment.Spec.Replicas) &&
		newStatus.Replicas == *(deployment.Spec.Replicas) &&
		newStatus.AvailableReplicas == *(deployment.Spec.Replicas) &&
		newStatus.ObservedGeneration >= deployment.Generation
}
