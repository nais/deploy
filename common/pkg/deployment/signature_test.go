package deployment_test

import (
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/stretchr/testify/assert"
	"testing"
)

const (
	secureKey       = "super-secure-key"
	impersonatedKey = "not-my-key"
)

func TestSignature(t *testing.T) {
	a := assert.New(t)
	msg := &deployment.DeploymentRequest{
		Cluster: "foo",
		Payload: []byte("bar"),
	}

	signedMessage, err := deployment.SignMessage(msg, secureKey)
	a.Nil(err)
	a.NotNil(signedMessage)

	err = deployment.CheckMessageSignature(*signedMessage, secureKey)
	a.Nil(err)

	err = deployment.CheckMessageSignature(*signedMessage, impersonatedKey)
	a.EqualError(err, deployment.ErrSignaturesDiffer.Error())

	bogusSignedMessage := &deployment.SignedMessage{
		Message:   signedMessage.Message,
		Signature: []byte(secureKey),
	}
	err = deployment.CheckMessageSignature(*bogusSignedMessage, secureKey)
	a.EqualError(err, deployment.ErrSignaturesDiffer.Error())
}
