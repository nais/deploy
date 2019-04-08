package deployment_test

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/stretchr/testify/assert"
)

const (
	secureKey       = "super-secure-key"
	impersonatedKey = "not-my-key"
)

func TestSignature(t *testing.T) {
	a := assert.New(t)
	msg := &deployment.DeploymentRequest{
		Cluster:     "foo",
		PayloadSpec: &deployment.Payload{},
	}

	payload, err := deployment.WrapMessage(msg, secureKey)
	a.Nil(err)
	a.NotNil(payload)

	unwrapped := &deployment.DeploymentRequest{}
	err = deployment.UnwrapMessage(payload, secureKey, unwrapped)
	a.Nil(err)
	a.True(proto.Equal(msg, unwrapped))

	err = deployment.UnwrapMessage(payload, impersonatedKey, unwrapped)
	a.NotNil(err)
}
