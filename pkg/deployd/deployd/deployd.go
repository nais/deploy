package deployd

import (
	"fmt"
	"sync"

	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/metrics"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/deployd/strategy"
	"github.com/nais/deploy/pkg/k8sutils"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
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
		return
	}

	resources, err := op.ExtractResources()
	if err != nil {
		failure(err)
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

		resourceInterface, err := client.ResourceInterface(&resource)
		if err == nil {
			_, err = strategy.NewDeployStrategy(resourceInterface).Deploy(op.Context, resource)
		}

		if err != nil {
			err = fmt.Errorf("%s: %s", identifier.String(), err)
			logger.Error(err)
			errors <- err
			break
		}

		metrics.KubernetesResources.Inc()

		op.StatusChan <- pb.NewInProgressStatus(op.Request, "Successfully applied %s", identifier.String())
		wait.Add(1)

		go func(logger *log.Entry, resource unstructured.Unstructured) {
			deadline, _ := op.Context.Deadline()
			op.Logger.Debugf("Monitoring rollout status of '%s/%s' in namespace '%s', deadline %s", identifier.GroupVersionKind, identifier.Name, identifier.Namespace, deadline)
			strat := strategy.NewWatchStrategy(identifier.GroupVersionKind, client)
			status := strat.Watch(op, resource)
			if status != nil {
				if status.GetState().IsError() {
					errors <- fmt.Errorf(status.Message)
					op.Logger.Error(status.Message)
				} else {
					op.Logger.Infof(status.Message)
				}
				status.State = pb.DeploymentState_in_progress
				op.StatusChan <- status
			}

			op.Logger.Debugf("Finished monitoring rollout status of '%s/%s' in namespace '%s'", identifier.GroupVersionKind, identifier.Name, identifier.Namespace)
			wait.Done()
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
			op.StatusChan <- pb.NewFailureStatus(op.Request, fmt.Errorf("%s (total of %d errors)", err, errCount))
		} else {
			op.StatusChan <- pb.NewSuccessStatus(op.Request)
		}
	}()
}
