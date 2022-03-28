package strategy

import (
	"context"
	"fmt"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"github.com/nais/liberator/pkg/events"
	"regexp"
	"time"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type naisResource struct {
	client kubeclient.Interface
}

func (a naisResource) Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	var err error

	eventsClient := a.client.Kubernetes().CoreV1().Events(resource.GetNamespace())
	deadline, _ := op.Context.Deadline()
	timeoutSecs := int64(deadline.Sub(time.Now()).Seconds())
	ctx, cancel := context.WithCancel(op.Context)
	defer cancel()

	eventWatcher, err := eventsClient.Watch(ctx, metav1.ListOptions{
		TimeoutSeconds:  &timeoutSecs,
		ResourceVersion: "0",
	})

	if err != nil {
		return pb.NewErrorStatus(op.Request, fmt.Errorf("unable to set up event watcher: %w", err))
	}

	defer eventWatcher.Stop()
	watchStart := time.Now().Truncate(time.Second)

	for {
		select {
		case watchEvent, ok := <-eventWatcher.ResultChan():
			if !ok {
				return pb.NewErrorStatus(op.Request, ErrDeploymentTimeout)
			}

			event, ok := watchEvent.Object.(*v1.Event)
			if !ok {
				// failed cast
				op.Logger.Errorf("Event is of wrong type: %T", watchEvent)
				continue
			}

			if !EventStreamMatch(event, resource.GetName()) {
				op.Logger.Tracef("Ignoring unrelated event %v", event.Name)
				continue
			}

			if event.LastTimestamp.Time.Before(watchStart) {
				op.Logger.Tracef("Ignoring old event %s", event.Name)
				continue
			}

			status := StatusFromEvent(event, op.Request)
			if status == nil {
				return pb.NewFailureStatus(op.Request, fmt.Errorf("this application has been redeployed, aborting monitoring"))
			}

			if status.GetState().Finished() {
				return status
			}

			op.StatusChan <- status

		case <-op.Context.Done():
			return pb.NewErrorStatus(op.Request, ErrDeploymentTimeout)
		}
	}
}

func EventString(event *v1.Event) string {
	return fmt.Sprintf("%s/%s (%s): %s", event.InvolvedObject.Kind, event.InvolvedObject.Name, event.Reason, event.Message)
}

func StatusFromEvent(event *v1.Event, req *pb.DeploymentRequest) *pb.DeploymentStatus {
	status := &pb.DeploymentStatus{
		Request: req,
		Message: EventString(event),
		State:   pb.DeploymentState_in_progress,
		Time:    pb.TimeAsTimestamp(time.Now()),
	}

	if event.ReportingController == "naiserator" {
		id, _ := event.GetAnnotations()[nais_io_v1.DeploymentCorrelationIDAnnotation]
		if id != status.GetRequest().GetID() {
			return nil // not a status that applies to our request id
		}
		switch event.Reason {
		case events.FailedPrepare:
			fallthrough
		case events.FailedSynchronization:
			status.State = pb.DeploymentState_failure
		case events.RolloutComplete:
			status.State = pb.DeploymentState_success
		}
	}

	return status
}

func EventStreamMatch(event *v1.Event, resourceName string) bool {
	var re string
	switch event.InvolvedObject.Kind {
	case "Pod":
		re = fmt.Sprintf(`^%s-([a-z0-9]{10}-)?[a-z0-9]{5}$`, resourceName)
	case "ReplicaSet":
		re = fmt.Sprintf(`^%s-[a-z0-9]{10}$`, resourceName)
	case "Job":
		re = fmt.Sprintf(`^%s(-[a-z0-9]{5})?$`, resourceName)
	default:
		re = fmt.Sprintf(`^%s$`, resourceName)
	}
	matched, _ := regexp.MatchString(re, event.InvolvedObject.Name)
	return matched
}
