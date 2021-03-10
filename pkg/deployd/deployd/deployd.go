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

func Run(op *operation.Operation) {
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

	for index, resource := range resources {
		addCorrelationID(&resource, op.Request.GetID())
		identifier := k8sutils.ResourceIdentifier(resource)

		op.Logger = op.Logger.WithFields(log.Fields{
			"name":      identifier.Name,
			"namespace": identifier.Namespace,
			"gvk":       identifier.GroupVersionKind,
		})

		deployed, err := op.TeamClient.DeployUnstructured(resource)
		if err != nil {
			err = fmt.Errorf("resource %d: %s", index+1, err)
			op.Logger.Error(err)
			errors <- err
			break
		}

		metrics.KubernetesResources.Inc()

		op.Logger.Infof("Resource %d: successfully deployed %s", index+1, deployed.GetSelfLink())
		wait.Add(1)

		go func(logger *log.Entry, resource unstructured.Unstructured) {
			deadline, _ := op.Context.Deadline()
			op.Logger.Infof("Monitoring rollout status of '%s/%s' in namespace '%s', deadline %s", identifier.GroupVersionKind, identifier.Name, identifier.Namespace, deadline)
			err := op.TeamClient.WaitForDeployment(op.Context, op.Logger, resource, op.Request, op.StatusChan)
			if err != nil {
				op.Logger.Error(err)
				errors <- err
			}
			op.Logger.Infof("Finished monitoring rollout status of '%s/%s' in namespace '%s'", identifier.GroupVersionKind, identifier.Name, identifier.Namespace)
			wait.Done()
		}(op.Logger, resource)
	}

	op.StatusChan <- pb.NewInProgressStatus(op.Request)

	go func() {
		op.Logger.Infof("Waiting for resources to be successfully rolled out")
		wait.Wait()
		op.Logger.Infof("Finished monitoring all resources")

		errCount := len(errors)
		if errCount == 0 {
			op.StatusChan <- pb.NewSuccessStatus(op.Request)
		} else {
			err := <-errors
			op.StatusChan <- pb.NewFailureStatus(op.Request, fmt.Errorf("%s (total of %d errors)", err, errCount))
		}
	}()
}
