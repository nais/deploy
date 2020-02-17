package strategy

import (
	"fmt"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

type deployment struct {
	client kubernetes.Interface
}

func (d deployment) Watch(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error {
	var cur *apps.Deployment
	var nova *apps.Deployment
	var err error
	var resourceVersion int
	var updated bool

	logger = logger.WithFields(log.Fields{
		"deployment": resource.GetName(),
		"namespace":  resource.GetNamespace(),
	})

	client := d.client.AppsV1().Deployments(resource.GetNamespace())

	// For native Kubernetes deployment objects, get the current deployment object.
	for deadline.After(time.Now()) {
		cur, err = client.Get(resource.GetName(), metav1.GetOptions{})
		if err == nil {
			resourceVersion, _ = strconv.Atoi(cur.GetResourceVersion())
			logger.Tracef("Found current deployment at version %d: %s", resourceVersion, cur.GetSelfLink())
		} else if errors.IsNotFound(err) {
			logger.Tracef("Deployment '%s' in namespace '%s' is not currently present in the cluster.", resource.GetName(), resource.GetNamespace())
		} else {
			logger.Tracef("Recoverable error while polling for deployment object: %s", err)
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
			logger.Tracef("New deployment appeared at version %d: %s", rv, cur.GetSelfLink())
			resourceVersion = rv
			updated = true
		}

		if updated && deploymentComplete(nova, &nova.Status) {
			return nil
		}

		logger.WithFields(log.Fields{
			"deployment_replicas":            nova.Status.Replicas,
			"deployment_updated_replicas":    nova.Status.UpdatedReplicas,
			"deployment_available_replicas":  nova.Status.AvailableReplicas,
			"deployment_observed_generation": nova.Status.ObservedGeneration,
		}).Tracef("Still waiting for deployment to finish rollout...")

		time.Sleep(requestInterval)
	}

	if err != nil {
		return fmt.Errorf("%s; last error was: %s", ErrDeploymentTimeout, err)
	}

	return ErrDeploymentTimeout
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
