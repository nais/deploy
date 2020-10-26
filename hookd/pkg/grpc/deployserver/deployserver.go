package deployserver

import (
	"context"
	"fmt"

	"github.com/navikt/deployment/common/pkg/deployment"
)

type DeployServer struct {
}

var _ deployment.DeployServer = &DeployServer{}

func (s *DeployServer) Deployments(*deployment.GetDeploymentOpts, deployment.Deploy_DeploymentsServer) error {
	return fmt.Errorf("not implemented")
}

func (s *DeployServer) ReportStatus(context.Context, *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, fmt.Errorf("not implemented")
}
