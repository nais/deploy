package api_v1_deploy_test

import (
	"testing"

	server "github.com/nais/deploy/pkg/hookd/api/v1/deploy"
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
	deploymentRequest := &server.DeploymentRequest{}
	deploymentRequest.Resources = []byte(`[{"foo": "bar"}]`)
	correlationID := "bar"

	deployMsg, err := server.DeploymentRequestMessage(deploymentRequest, correlationID)

	assert.NoError(t, err)

	resources := deployMsg.GetKubernetes().GetResources()
	val := resources[0].Fields["foo"]

	assert.Len(t, resources, 1)
	assert.NotNil(t, val)
	assert.Equal(t, "bar", val.GetStringValue())
}
