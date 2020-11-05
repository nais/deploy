package pb_test

import (
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/pkg/pb"
	"github.com/stretchr/testify/assert"
)

const (
	secureKey       = "super-secure-key"
	impersonatedKey = "not-my-key"
)

func TestSignature(t *testing.T) {
	a := assert.New(t)
	msg := &pb.DeploymentRequest{
		Cluster:     "foo",
		PayloadSpec: &pb.Payload{},
	}

	payload, err := pb.WrapMessage(msg, secureKey)
	a.Nil(err)
	a.NotNil(payload)

	unwrapped := &pb.DeploymentRequest{}
	err = pb.UnwrapMessage(payload, secureKey, unwrapped)
	a.Nil(err)
	a.True(proto.Equal(msg, unwrapped))

	err = pb.UnwrapMessage(payload, impersonatedKey, unwrapped)
	a.NotNil(err)
}
