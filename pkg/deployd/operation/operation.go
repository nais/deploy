package operation

import (
	"context"
	"fmt"

	"github.com/navikt/deployment/pkg/k8sutils"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Operation struct {
	Context    context.Context
	Logger     *log.Entry
	Request    *pb.DeploymentRequest
	StatusChan chan<- *pb.DeploymentStatus
}

func (op *Operation) ExtractResources() ([]unstructured.Unstructured, error) {
	resources, err := k8sutils.ResourcesFromDeploymentRequest(op.Request)
	if err != nil {
		return nil, err
	}

	if len(resources) == 0 {
		return nil, fmt.Errorf("no resources to deploy")
	}

	return resources, nil
}
