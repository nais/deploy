package strategy

import (
	"fmt"
	"time"

	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type job struct {
	client kubeclient.Interface
}

func (j job) Watch(op *operation.Operation, resource unstructured.Unstructured) *pb.DeploymentStatus {
	var job *v1.Job
	var err error

	client := j.client.Kubernetes().BatchV1().Jobs(resource.GetNamespace())
	deadline, _ := op.Context.Deadline()

	// Wait until the new job object is present in the cluster.
	for deadline.After(time.Now()) {
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
		return pb.NewErrorStatus(op.Request, fmt.Errorf("%s; last error was: %s", ErrDeploymentTimeout, err))
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
