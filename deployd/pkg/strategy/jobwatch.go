package strategy

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

type job struct {
	client kubernetes.Interface
}

func (j job) Watch(logger *log.Entry, resource unstructured.Unstructured, deadline time.Time) error {
	var job *v1.Job
	var err error

	logger = logger.WithFields(log.Fields{
		"job":       resource.GetName(),
		"namespace": resource.GetNamespace(),
	})

	client := j.client.BatchV1().Jobs(resource.GetNamespace())

	// Wait until the new job object is present in the cluster.
	for deadline.After(time.Now()) {
		job, err = client.Get(resource.GetName(), metav1.GetOptions{})

		if err != nil {
			time.Sleep(requestInterval)
			continue
		}

		if jobComplete(job) {
			return nil
		}
		logger.Tracef("Still waiting for job to complete...")

		time.Sleep(requestInterval)
	}

	if err != nil {
		return fmt.Errorf("%s; last error was: %s", ErrDeploymentTimeout, err)
	}
	return ErrDeploymentTimeout
}

func jobComplete(job *v1.Job) bool {
	for _, condition := range job.Status.Conditions {
		if condition.Type == v1.JobComplete {
			return true
		}
	}
	return false
}
