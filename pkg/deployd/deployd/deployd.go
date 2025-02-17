package deployd

import (
	"fmt"
	"sync"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/metrics"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/deployd/strategy"
	"github.com/nais/deploy/pkg/k8sutils"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	otrace "go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Annotate a resource with the deployment correlation ID.
func addCorrelationID(resource *unstructured.Unstructured, correlationID string) {
	anno := resource.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	anno[nais_io_v1.DeploymentCorrelationIDAnnotation] = correlationID
	resource.SetAnnotations(anno)
}

func Run(op *operation.Operation, client kubeclient.Interface) {
	op.Logger.Infof("Starting deployment")

	failure := func(err error) {
		op.Cancel()
		op.StatusChan <- pb.NewFailureStatus(op.Request, err)
	}

	err := op.Context.Err()
	if err != nil {
		failure(err)
		op.Trace.SetStatus(codes.Error, err.Error())
		op.Trace.End()
		return
	}

	resources, err := op.ExtractResources()
	if err != nil {
		failure(err)
		op.Trace.SetStatus(codes.Error, err.Error())
		op.Trace.End()
		return
	}

	wait := sync.WaitGroup{}
	errors := make(chan error, len(resources))

	for _, resource := range resources {
		addCorrelationID(&resource, op.Request.GetID())
		identifier := k8sutils.ResourceIdentifier(resource)

		logger := op.Logger.WithFields(log.Fields{
			"name":      identifier.Name,
			"namespace": identifier.Namespace,
			"gvk":       identifier.GroupVersionKind,
		})

		spanName := fmt.Sprintf("%s/%s", identifier.Kind, identifier.Name)
		_, span := telemetry.Tracer().Start(op.Context, spanName, otrace.WithSpanKind(otrace.SpanKindClient))
		telemetry.AddDeploymentRequestSpanAttributes(span, op.Request)
		span.SetAttributes(
			attribute.KeyValue{
				Key:   "k8s.kind",
				Value: attribute.StringValue(identifier.Kind),
			},
			attribute.KeyValue{
				Key:   "k8s.name",
				Value: attribute.StringValue(identifier.Name),
			},
			attribute.KeyValue{
				Key:   "k8s.namespace",
				Value: attribute.StringValue(identifier.Namespace),
			},
			attribute.KeyValue{
				Key:   "k8s.apiVersion",
				Value: attribute.StringValue(identifier.GroupVersion().String()),
			},
		)

		resourceInterface, err := client.ResourceInterface(&resource)
		if err == nil {
			_, err = strategy.NewDeployStrategy(resourceInterface).Deploy(op.Context, resource, span)
		}

		if err != nil {
			err = fmt.Errorf("%s: %s", identifier.String(), err)
			span.SetStatus(codes.Error, err.Error())
			span.End()
			logger.Error(err)
			errors <- err
			break
		}

		span.AddEvent("Resource saved to Kubernetes")

		metrics.KubernetesResources(op.Request.GetTeam(), identifier.Kind, identifier.Name).Inc()

		op.StatusChan <- pb.NewInProgressStatus(op.Request, "Successfully applied %s", identifier.String())
		wait.Add(1)

		go func(logger *log.Entry, resource unstructured.Unstructured) {
			deadline, _ := op.Context.Deadline()
			op.Logger.Debugf("Monitoring rollout status of '%s/%s' in namespace '%s', deadline %s", identifier.GroupVersionKind, identifier.Name, identifier.Namespace, deadline)
			strat := strategy.NewWatchStrategy(identifier.GroupVersionKind, client)
			status := strat.Watch(op, resource, span)
			if status != nil {
				span.AddEvent(status.Message)
				if status.GetState().IsError() {
					span.SetStatus(codes.Error, status.Message)
					errors <- fmt.Errorf("%s", status.Message)
					op.Logger.Error(status.Message)
				} else {
					span.SetStatus(codes.Ok, status.Message)
					op.Logger.Info(status.Message)
				}
				status.State = pb.DeploymentState_in_progress
				op.StatusChan <- status
			} else {
				span.SetStatus(codes.Ok, "Resource saved to Kubernetes")
			}

			op.Logger.Debugf("Finished monitoring rollout status of '%s/%s' in namespace '%s'", identifier.GroupVersionKind, identifier.Name, identifier.Namespace)
			wait.Done()
			span.End()
		}(logger, resource)
	}

	op.StatusChan <- pb.NewInProgressStatus(op.Request, "All resources saved to Kubernetes; waiting for completion")

	go func() {
		op.Logger.Debugf("Waiting for resources to be successfully rolled out")
		wait.Wait()
		op.Logger.Debugf("Finished monitoring all resources")
		op.Cancel()

		errCount := len(errors)
		if errCount > 0 {
			err := <-errors
			close(errors)
			aggregateError := fmt.Errorf("%s (total of %d errors)", err, errCount)
			op.StatusChan <- pb.NewFailureStatus(op.Request, aggregateError)
			op.Trace.SetStatus(codes.Error, aggregateError.Error())
		} else {
			op.StatusChan <- pb.NewSuccessStatus(op.Request)
			op.Trace.SetStatus(codes.Ok, "All resources rolled out successfully")
		}

		op.Trace.End()
	}()
}
