package deployd

import (
	"fmt"
	"sync"

	"github.com/navikt/deployment/pkg/deployd/kubeclient"
	"github.com/navikt/deployment/pkg/deployd/metrics"
	"github.com/navikt/deployment/pkg/deployd/operation"
	"github.com/navikt/deployment/pkg/k8sutils"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Annotate a resource with the deployment correlation ID.
func addCorrelationID(resource *unstructured.Unstructured, correlationID string) {
	anno := resource.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	anno[kubeclient.CorrelationIDAnnotation] = correlationID
	resource.SetAnnotations(anno)
}

func Run(op *operation.Operation, client kubeclient.TeamClient) {
	op.Logger.Infof("Starting deployment")

	failure := func(err error) {
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

		op.Logger = op.Logger.WithFields(log.Fields{
			"name":      identifier.Name,
			"namespace": identifier.Namespace,
			"gvk":       identifier.GroupVersionKind,
		})

		deployed, err := client.DeployUnstructured(resource)
		if err != nil {
			err = fmt.Errorf("%s: %s", resource.GetSelfLink(), err)
			op.Logger.Error(err)
			errors <- err
			break
		}

		metrics.KubernetesResources.Inc()

		op.StatusChan <- pb.NewInProgressStatus(op.Request, "Successfully applied %s", deployed.GetSelfLink())

		go func(logger *log.Entry, resource unstructured.Unstructured) {
			wait.Add(1)
			deadline, _ := op.Context.Deadline()
			op.Logger.Debugf("Monitoring rollout status of '%s/%s' in namespace '%s', deadline %s", identifier.GroupVersionKind, identifier.Name, identifier.Namespace, deadline)
			status := client.WaitForDeployment(op, resource)
			if status != nil {
				if status.GetState().IsError() {
					errors <- fmt.Errorf(status.Message)
					op.Logger.Error(err)
				} else {
					op.Logger.Infof(status.Message)
				}
				status.State = pb.DeploymentState_in_progress
				op.StatusChan <- status
			}

			op.Logger.Debugf("Finished monitoring rollout status of '%s/%s' in namespace '%s'", identifier.GroupVersionKind, identifier.Name, identifier.Namespace)
			wait.Done()
		}(op.Logger, resource)
	}

	op.StatusChan <- pb.NewInProgressStatus(op.Request, "All resources saved to Kubernetes; waiting for completion")

	go func() {
		op.Logger.Debugf("Waiting for resources to be successfully rolled out")
		wait.Wait()
		op.Logger.Debugf("Finished monitoring all resources")

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
