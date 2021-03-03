package strategy

import (
	"context"
	"fmt"
	"regexp"
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

func (a application) Watch(ctx context.Context, logger *log.Entry, resource unstructured.Unstructured, request *pb.DeploymentRequest, statusChan chan<- *pb.DeploymentStatus) error {
	var err error

	eventsClient := a.structuredClient.CoreV1().Events(resource.GetNamespace())
	deadline, _ := ctx.Deadline()
	timeoutSecs := int64(deadline.Sub(time.Now()).Seconds())
	eventWatcher, err := eventsClient.Watch(metav1.ListOptions{
		TimeoutSeconds:  &timeoutSecs,
		ResourceVersion: "0",
	})

	if err != nil {
		return fmt.Errorf("unable to set up event watcher: %w", err)
	}

	watchStart := time.Now().Truncate(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		select {
		case watchEvent := <-eventWatcher.ResultChan():
			event, ok := watchEvent.Object.(*v1.Event)
			if !ok {
				// failed cast
				logger.Errorf("Event is of wrong type: %T", watchEvent)
				continue
			}

			if !EventStreamMatch(event, resource.GetName()) {
				logger.Tracef("Ignoring unrelated event %s", event.Name)
				continue
			}

			if event.LastTimestamp.Time.Before(watchStart) {
				logger.Tracef("Ignoring old event %s", event.Name)
				continue
			}

			status := StatusFromEvent(event, request)
			if status == nil {
				return fmt.Errorf("this application has been redeployed, aborting monitoring")
			}

			switch status.State {
			case pb.DeploymentState_success:
				return nil
			case pb.DeploymentState_failure:
				return fmt.Errorf(status.Message)
			}
			statusChan <- status

		case <-ctx.Done():
			return ErrDeploymentTimeout
		}
	}
}
