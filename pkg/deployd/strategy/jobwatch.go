package strategy

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	"go.opentelemetry.io/otel/trace"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type job struct {
	client kubeclient.Interface
}

func (j job) Watch(op *operation.Operation, resource unstructured.Unstructured, trace trace.Span) *pb.DeploymentStatus {
	var job *v1.Job
	var err error

	client := j.client.Kubernetes().BatchV1().Jobs(resource.GetNamespace())

	ctx, cancel := context.WithCancel(op.Context)
	defer cancel()

	// Wait until the new job object is present in the cluster.
	for ctx.Err() == nil {
		job, err = client.Get(op.Context, resource.GetName(), metav1.GetOptions{})

		if err != nil {
			time.Sleep(requestInterval)
			continue
		}

		if jobComplete(job) {
			return nil
		}

		if status, condition := jobFailed(job); status {
			return pb.NewFailureStatus(op.Request, fmt.Errorf("job failed: %s", condition.String()))
		}

		op.Logger.Debugf("Still waiting for job to complete...")

		time.Sleep(requestInterval)
	}

	if err != nil {
		err = fmt.Errorf("%s; last error was: %w", ErrDeploymentTimeout, err)
		trace.AddEvent(err.Error())
		return pb.NewErrorStatus(op.Request, err)
	}

	return pb.NewErrorStatus(op.Request, ErrDeploymentTimeout)
}

func jobComplete(job *v1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == v1.JobComplete {
			return true
		}
	}
	return false
}

func jobFailed(job *v1.Job) (bool, v1.JobCondition) {
	for _, condition := range job.Status.Conditions {
		if condition.Type == v1.JobFailed {
			return true, condition
		}
	}
	return false, v1.JobCondition{}
}
