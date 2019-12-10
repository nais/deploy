package kubeclient

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	apps "k8s.io/api/apps/v1"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/restmapper"
)

var (
	requestInterval      = time.Second * 5
	ErrDeploymentTimeout = fmt.Errorf("timeout while waiting for deployment to succeed")
)

const (
	CorrelationIDAnnotation = "nais.io/deploymentCorrelationID"
)

const (
	EventRolloutComplete       = "RolloutComplete"
	EventFailedPrepare         = "FailedPrepare"
	EventFailedSynchronization = "FailedSynchronization"
)

type appStatus struct {
	CorrelationID        string
	SynchronizationState string
}

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
	return c.createOrUpdate(namespacedResource, resource)
}

// Retrieve the most recent application deployment event.
//
// Events are re-used by Naiserator, having their Count field incremented by one every time.
// This function retrieves the event with the specified Reason, and checks if the correlation ID
// annotation is set to the same value as the original resource.
func (c *teamClient) getApplicationEvent(resource unstructured.Unstructured, reason string) (*v1.Event, error) {
	eventClient := c.structuredClient.CoreV1().Events(resource.GetNamespace())

	selectors := []string{
		fmt.Sprintf("involvedObject.name=%s", resource.GetName()),
		fmt.Sprintf("involvedObject.namespace=%s", resource.GetNamespace()),
		fmt.Sprintf("reason=%s", reason),
		"involvedObject.kind=Application",
	}

	events, err := eventClient.List(metav1.ListOptions{
		FieldSelector: strings.Join(selectors, ","),
		Limit:         1,
	})

	if err != nil {
		return nil, err
	}

	if events == nil || len(events.Items) == 0 {
		return nil, fmt.Errorf("no events found")
	}

	event := &events.Items[0]

	if event.Annotations == nil {
		return nil, fmt.Errorf("event annotation list is empty")
	}

	if event.Annotations[CorrelationIDAnnotation] == resource.GetAnnotations()[CorrelationIDAnnotation] {
		return nil, fmt.Errorf("event correlation ID does not match")
	}

	return event, nil
}

func parseAppStatus(resource unstructured.Unstructured) *appStatus {
	data, ok := resource.Object["status"]
	if !ok {
		return nil
	}
	datamap, ok := data.(map[string]interface{})
	if !ok {
		return nil
	}

	st := &appStatus{}
	st.SynchronizationState, _ = datamap["synchronizationState"].(string)
	st.CorrelationID, _ = datamap["correlationID"].(string)
	return st
}

func (c *teamClient) waitForApplication(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error {
	var updated *unstructured.Unstructured
	var err error
	var status *appStatus

	correlationID, _ := resource.GetAnnotations()[CorrelationIDAnnotation]

	gvk := resource.GroupVersionKind()
	appcli := c.unstructuredClient.Resource(schema.GroupVersionResource{
		Resource: "applications",
		Version:  gvk.Version,
		Group:    gvk.Group,
	}).Namespace(resource.GetNamespace())

	for deadline.After(time.Now()) {
		updated, err = appcli.Get(resource.GetName(), metav1.GetOptions{})

		if err != nil {
			logger.Tracef("Retrieving updated Application resource %s: %s", resource.GetSelfLink(), err)
			goto NEXT
		}

		status = parseAppStatus(*updated)
		if status.CorrelationID != correlationID {
			logger.Tracef("Application correlation ID mismatch; not picked up by Naiserator yet.")
			goto NEXT
		}

		logger.Tracef("Application synchronization state: '%s'", status.SynchronizationState)

		switch status.SynchronizationState {
		case EventRolloutComplete:
			return nil

		case EventFailedSynchronization, EventFailedPrepare:
			event, err := c.getApplicationEvent(*updated, status.SynchronizationState)
			if err != nil {
				logger.Errorf("Get application event: %s", err)
				return fmt.Errorf(status.SynchronizationState)
			}
			return fmt.Errorf("%s", event.Message)
		}

	NEXT:
		time.Sleep(requestInterval)
		continue
	}

	return ErrDeploymentTimeout
}

// Returns nil after the next generation of the deployment is successfully rolled out,
// or error if it has not succeeded within the specified deadline.
func (c *teamClient) WaitForDeployment(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error {
	var cur *apps.Deployment
	var nova *apps.Deployment
	var err error
	var resourceVersion int
	var updated bool

	cli := c.structuredClient.AppsV1().Deployments(resource.GetNamespace())

	// For Naiserator applications, rely on Naiserator set a terminal rollout status.
	gvk := resource.GroupVersionKind()
	if gvk.Kind == "Application" && gvk.Group == "nais.io" {
		return c.waitForApplication(logger, resource, deadline)
	}

	// For native Kubernetes deployment objects, get the current deployment object.
	for deadline.After(time.Now()) {
		cur, err = cli.Get(resource.GetName(), metav1.GetOptions{})
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
		nova, err = cli.Get(resource.GetName(), metav1.GetOptions{})
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

func (c *teamClient) createOrUpdate(client dynamic.ResourceInterface, resource unstructured.Unstructured) (*unstructured.Unstructured, error) {
	deployed, err := client.Create(&resource, metav1.CreateOptions{})
	if !errors.IsAlreadyExists(err) {
		return deployed, err
	}

	existing, err := client.Get(resource.GetName(), metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get existing resource: %s", err)
	}
	resource.SetResourceVersion(existing.GetResourceVersion())
	return client.Update(&resource, metav1.UpdateOptions{})
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
