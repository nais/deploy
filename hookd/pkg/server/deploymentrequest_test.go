package server_test

import (
	"testing"

	gh "github.com/google/go-github/v23/github"
	"github.com/navikt/deployment/hookd/pkg/server"
	"github.com/stretchr/testify/assert"
)

const (
	repoName    = "foo/bar"
	team        = "my team"
	deliveryID  = "delivery id"
	environment = "some environment"
	payload     = `{"team":"my team","kubernetes":{"resources":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"labels":{"team":"my team"},"name":"foobar","namespace":"default"}}]}}`
)

func TestDeploymentRequest(t *testing.T) {
	t.Run("well-formed deployment event returns a deployment request", func(t *testing.T) {
		ev := &gh.DeploymentEvent{
			Repo: &gh.Repository{
				FullName: gh.String(repoName),
			},
			Deployment: &gh.Deployment{
				Environment: gh.String(environment),
				Payload:     []byte(payload),
			},
		}
		req, err := server.DeploymentRequest(ev, deliveryID)
		assert.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, deliveryID, req.GetDeliveryID())
		assert.Equal(t, environment, req.GetCluster())
		assert.Equal(t, repoName, req.GetDeployment().GetRepository().FullName())
		assert.Equal(t, team, req.GetPayloadSpec().GetTeam())
	})
}
