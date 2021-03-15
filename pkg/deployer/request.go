package deployer

import (
	"time"

	"github.com/nais/deploy/pkg/pb"
)

func MakeDeploymentRequest(cfg Config, deadline time.Time, kubernetes *pb.Kubernetes) *pb.DeploymentRequest {
	return &pb.DeploymentRequest{
		Cluster:           cfg.Cluster,
		Deadline:          pb.TimeAsTimestamp(deadline),
		GitRefSha:         cfg.Ref,
		GithubEnvironment: cfg.Environment,
		Kubernetes:        kubernetes,
		Repository: &pb.GithubRepository{
			Owner: cfg.Owner,
			Name:  cfg.Repository,
		},
		Team: cfg.Team,
		Time: pb.TimeAsTimestamp(time.Now()),
	}
}
