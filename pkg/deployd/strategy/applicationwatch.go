package strategy

import (
	"context"
	"fmt"
	"regexp"
	"time"

	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/nais/liberator/pkg/events"
	"github.com/navikt/deployment/pkg/deployd/operation"
	"github.com/navikt/deployment/pkg/pb"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type application struct {
	structuredClient   kubernetes.Interface
	unstructuredClient dynamic.Interface
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
		id, _ := event.GetAnnotations()[nais_io_v1alpha1.DeploymentCorrelationIDAnnotation]
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
		re = fmt.Sprintf(`^%s-[a-z0-9]{10}-[a-z0-9]{5}$`, resourceName)
	case "ReplicaSet":
		re = fmt.Sprintf(`^%s-[a-z0-9]{10}$`, resourceName)
	default:
		re = fmt.Sprintf(`^%s$`, resourceName)
	}
	matched, _ := regexp.MatchString(re, event.InvolvedObject.Name)
	return matched
}

func (a application) Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	var err error

	eventsClient := a.structuredClient.CoreV1().Events(resource.GetNamespace())
	deadline, _ := op.Context.Deadline()
	timeoutSecs := int64(deadline.Sub(time.Now()).Seconds())
	eventWatcher, err := eventsClient.Watch(metav1.ListOptions{
		TimeoutSeconds:  &timeoutSecs,
		ResourceVersion: "0",
	})

	if err != nil {
		return pb.NewErrorStatus(op.Request, fmt.Errorf("unable to set up event watcher: %w", err))
	}

	watchStart := time.Now().Truncate(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		select {
		case watchEvent, ok := <-eventWatcher.ResultChan():
			if !ok {
				logger.Tracef("Event watcher channel closed")
				return ErrDeploymentTimeout
			}

			event, ok := watchEvent.Object.(*v1.Event)
			if !ok {
				// failed cast
				op.Logger.Errorf("Event is of wrong type: %T", watchEvent)
				continue
			}

			if !EventStreamMatch(event, resource.GetName()) {
				op.Logger.Tracef("Ignoring unrelated event %s", event.Name)
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

		case <-ctx.Done():
			return pb.NewErrorStatus(op.Request, ErrDeploymentTimeout)
		}
	}
}
