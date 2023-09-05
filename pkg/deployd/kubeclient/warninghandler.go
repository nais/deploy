package kubeclient

import (
	"context"
	"fmt"
	"hash/crc32"
	"strings"
	"time"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/kubernetes"

	"github.com/nais/deploy/pkg/deployd/metrics"
	"github.com/nais/deploy/pkg/k8sutils"
)

type warningHandler struct {
	client        kubernetes.Interface
	correlationID string
	logger        *log.Entry
	resource      unstructured.Unstructured
}

func (w *warningHandler) HandleWarningHeader(_ int, _ string, message string) {
	// there doesn't appear to be any structured way of categorizing warnings, so this function will catch _all_ warnings.
	// this is also invoked for each individual warning; there can be multiple warnings when performing a single CRUD operation
	w.logger.Warnf("apiserver: %s", message)

	// only count relevant warnings
	if !strings.Contains(message, "unknown field") {
		return
	}

	metrics.DeployFieldValidationWarning(k8sutils.ResourceIdentifier(w.resource))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	eventsClient := w.client.CoreV1().Events(w.resource.GetNamespace())

	name, err := w.generateEventName(message)
	if err != nil {
		w.logger.Warnf("generating event name: %+v", err)
		return
	}

	event, err := eventsClient.Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err := eventsClient.Create(ctx, w.makeNewEvent(name, message), metav1.CreateOptions{})
		if err != nil {
			w.logger.Warnf("creating validation failure event: %+v", err)
		}
	} else if err != nil {
		w.logger.Warnf("getting validation failure event: %+v", err)
	} else {
		_, err = eventsClient.Update(ctx, w.makeUpdateEvent(*event), metav1.UpdateOptions{})
		if err != nil {
			w.logger.Warnf("updating validation failure event: %+v", err)
		}
	}
}

func (w *warningHandler) makeNewEvent(name, message string) *v1.Event {
	t := metav1.NewTime(time.Now())

	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: w.resource.GetNamespace(),
			Annotations: map[string]string{
				nais_io_v1.DeploymentCorrelationIDAnnotation: w.correlationID,
			},
		},
		InvolvedObject: v1.ObjectReference{
			Kind:            w.resource.GetKind(),
			Namespace:       w.resource.GetNamespace(),
			Name:            w.resource.GetName(),
			UID:             w.resource.GetUID(),
			APIVersion:      w.resource.GetAPIVersion(),
			ResourceVersion: w.resource.GetResourceVersion(),
		},
		Reason:         "FailedValidation",
		Message:        fmt.Sprintf("%s: this field will not have any effect and should either be corrected or removed; ignoring validation failure for now...", message),
		FirstTimestamp: t,
		LastTimestamp:  t,
		Count:          1,
		Type:           v1.EventTypeWarning,
		Source: v1.EventSource{
			Component: "deployd",
		},
		ReportingController: "deployd",
		ReportingInstance:   "deployd",
	}
}

func (w *warningHandler) makeUpdateEvent(event v1.Event) *v1.Event {
	event.Count = event.Count + 1
	event.LastTimestamp = metav1.NewTime(time.Now())

	a := event.GetAnnotations()
	if a == nil {
		a = make(map[string]string)
	}
	a[nais_io_v1.DeploymentCorrelationIDAnnotation] = w.correlationID
	event.SetAnnotations(a)

	return &event
}

// generateEventName generates an event name consisting of the resource name and a hash of the given message to avoid generating duplicate events
// the resource name is truncated if the result would be longer than the max length permitted by the DNS validation rules in RFC 1035
func (w *warningHandler) generateEventName(message string) (string, error) {
	basename := w.resource.GetName()
	maxlen := validation.DNS1035LabelMaxLength - 9
	if len(basename) > maxlen {
		basename = basename[:maxlen]
	}

	hasher := crc32.NewIEEE()
	_, err := hasher.Write([]byte(message))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s.%08x", basename, hasher.Sum32()), nil
}
