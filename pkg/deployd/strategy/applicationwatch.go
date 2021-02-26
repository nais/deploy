package strategy

import (
	"context"
	"fmt"
	"strings"
	"time"

	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/nais/liberator/pkg/events"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	CorrelationIDAnnotation = "nais.io/deploymentCorrelationID"
)

type application struct {
	structuredClient   kubernetes.Interface
	unstructuredClient dynamic.Interface
}

type appStatus struct {
	CorrelationID        string
	SynchronizationState string
}

const (
	EventRolloutComplete       = "RolloutComplete"
	EventFailedPrepare         = "FailedPrepare"
	EventFailedSynchronization = "FailedSynchronization"
)

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

// Retrieve the most recent application deployment event.
//
// Events are re-used by Naiserator, having their Count field incremented by one every time.
// This function retrieves the event with the specified Reason, and checks if the correlation ID
// annotation is set to the same value as the original resource.
func (a application) getApplicationEvent(resource unstructured.Unstructured, reason string) (*v1.Event, error) {
	eventClient := a.structuredClient.CoreV1().Events(resource.GetNamespace())

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

func EventString(event *v1.Event) string {
	return fmt.Sprintf("%s (%s): %s", event.InvolvedObject.Kind, event.Reason, event.Message)
}

func StatusFromEvent(event *v1.Event, req *pb.DeploymentRequest) *pb.DeploymentStatus {
	status := &pb.DeploymentStatus{
		Cluster:     req.GetCluster(),
		DeliveryID:  req.GetDeliveryID(),
		Deployment:  req.Deployment,
		Description: EventString(event),
		State:       pb.GithubDeploymentState_in_progress,
		Team:        req.GetPayloadSpec().GetTeam(),
		Time:        pb.TimeAsTimestamp(time.Now()),
	}

	if event.ReportingController == "naiserator" {
		correlationID, _ := event.GetAnnotations()[nais_io_v1alpha1.DeploymentCorrelationIDAnnotation]
		if correlationID != req.DeliveryID {
			return nil
		}

		switch event.Reason {
		case events.FailedPrepare:
			fallthrough
		case events.FailedSynchronization:
			status.State = pb.GithubDeploymentState_failure
		case events.RolloutComplete:
			status.State = pb.GithubDeploymentState_success
		}
	}

	return status
}

func (a application) Watch(ctx context.Context, logger *log.Entry, resource unstructured.Unstructured, request *pb.DeploymentRequest, statusChan chan<- *pb.DeploymentStatus) error {
	// var updated *unstructured.Unstructured
	var err error
	// var status *appStatus
	// var pickedup bool

	// gvk := resource.GroupVersionKind()
	/*appcli := a.unstructuredClient.Resource(schema.GroupVersionResource{
		Resource: "applications",
		Version:  gvk.Version,
		Group:    gvk.Group,
	}).Namespace(resource.GetNamespace())
	*/
	eventsClient := a.structuredClient.CoreV1().Events(resource.GetNamespace())
	deadline, _ := ctx.Deadline()
	timeoutSecs := int64(deadline.Sub(time.Now()).Seconds())
	eventWatcher, err := eventsClient.Watch(metav1.ListOptions{
		FieldSelector:  fmt.Sprintf("involvedObject.name=%s", resource.GetName()),
		TimeoutSeconds: &timeoutSecs,
	})

	if err != nil {
		return fmt.Errorf("not able to fetch events, %w", err)
	}

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		select {
		case watchEvent := <-eventWatcher.ResultChan():
			event, ok := watchEvent.Object.(*v1.Event)
			if !ok {
				// failed cast
				continue
			}
			status := StatusFromEvent(event, request)
			if status == nil {
				continue
			}
			switch status.State {
			case pb.GithubDeploymentState_success:
				return nil
			case pb.GithubDeploymentState_failure:
				return fmt.Errorf(status.Description)
			}
			statusChan <- status

		case <-ctx.Done():
			return ErrDeploymentTimeout
		}
	}

	/*for deadline.After(time.Now()) {
		updated, err = appcli.Get(resource.GetName(), metav1.GetOptions{})

		if err != nil {
			logger.Tracef("Retrieving updated Application resource %s: %s", resource.GetSelfLink(), err)
			goto NEXT
		}

		status = parseAppStatus(*updated)
		if status == nil || status.CorrelationID != correlationID {
			if pickedup {
				return fmt.Errorf("Application resource has been overwritten, aborting monitoring.")
			}
			logger.Tracef("Application correlation ID mismatch; not picked up by Naiserator yet.")
			goto NEXT
		}

		pickedup = true
		logger.Tracef("Application synchronization state: '%s'", status.SynchronizationState)

		switch status.SynchronizationState {
		case EventRolloutComplete:
			return nil

		case EventFailedSynchronization, EventFailedPrepare:
			event, err := a.getApplicationEvent(*updated, status.SynchronizationState)
			if err != nil {
				logger.Errorf("Get application event: %s", err)
				return fmt.Errorf(status.SynchronizationState)
			}
			return fmt.Errorf("%s", event.Message)
		}

	NEXT:
		time.Sleep(requestInterval)
		continue
	}*/
}
