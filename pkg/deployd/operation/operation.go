package operation

import (
	"context"
	"fmt"

	"github.com/nais/deploy/pkg/k8sutils"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Operation struct {
	Context    context.Context
	Cancel     context.CancelFunc
	Logger     *log.Entry
	Request    *pb.DeploymentRequest
	Trace      trace.Span
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
