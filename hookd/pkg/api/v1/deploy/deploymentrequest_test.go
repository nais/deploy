package api_v1_deploy_test

import (
	"testing"

	gh "github.com/google/go-github/v27/github"
	server "github.com/navikt/deployment/hookd/pkg/api/v1/deploy"
	"github.com/stretchr/testify/assert"
)

const (
	repoName    = "foo/bar"
	team        = "my team"
	deliveryID  = "delivery id"
	environment = "some environment"
	payload     = `{"team":"my team","kubernetes":{"resources":[{"apiVersion":"v1","kind":"ConfigMap","metadata":{"labels":{"team":"my team"},"name":"foobar","namespace":"default"}}]}}`
)

func TestDeploymentRequestFromEvent(t *testing.T) {
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
		req, err := server.DeploymentRequestFromEvent(ev, deliveryID)
		assert.NoError(t, err)
		assert.NotNil(t, req)
		assert.Equal(t, deliveryID, req.GetDeliveryID())
		assert.Equal(t, environment, req.GetCluster())
		assert.Equal(t, repoName, req.GetDeployment().GetRepository().FullName())
		assert.Equal(t, team, req.GetPayloadSpec().GetTeam())
	})
}

func TestDeploymentRequest(t *testing.T) {
	deploymentRequest := &server.DeploymentRequest{}
	deploymentRequest.Resources = []byte(`[{"foo": "bar"}]`)
	correlationID := "bar"

	deployMsg, err := server.DeploymentRequestMessage(deploymentRequest, correlationID)

	assert.NoError(t, err)

	resources := deployMsg.GetPayloadSpec().GetKubernetes().GetResources()
	val := resources[0].Fields["foo"]

	assert.Len(t, resources, 1)
	assert.NotNil(t, val)
	assert.Equal(t, "bar", val.GetStringValue())
}
