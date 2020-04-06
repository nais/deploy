package broker

import (
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/pkg/crypto"
)

type Serializer interface {
	Marshal(request deployment.DeploymentRequest) (*sarama.ProducerMessage, error)
	Unmarshal(message sarama.ConsumerMessage) (*deployment.DeploymentStatus, error)
}

// serializer provides conversion between deployment request/status objects
// and encrypted Kafka messages.
type serializer struct {
	topic         string
	encryptionKey []byte
}

func NewSerializer(topic string, encryptionKey []byte) Serializer {
	return &serializer{
		topic:         topic,
		encryptionKey: encryptionKey,
	}
}

func (s *serializer) Marshal(request deployment.DeploymentRequest) (*sarama.ProducerMessage, error) {
	payload, err := proto.Marshal(&request)
	if err != nil {
		return nil, fmt.Errorf("unable to marshal: %s", err)
	}

	ciphertext, err := crypto.Encrypt(payload, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("unable to encrypt: %s", err)
	}

	return &sarama.ProducerMessage{
		Topic:     s.topic,
		Value:     sarama.StringEncoder(ciphertext),
		Timestamp: time.Unix(request.GetTimestamp(), 0),
	}, nil
}

func (s *serializer) Unmarshal(message sarama.ConsumerMessage) (*deployment.DeploymentStatus, error) {
	payload, err := crypto.Decrypt(message.Value, s.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("unable to decrypt: %s", err)
	}

	status := deployment.DeploymentStatus{}
	err = proto.Unmarshal(payload, &status)
	if err != nil {
		return nil, fmt.Errorf("unable to unmarshal: %s", err)
	}

	return &status, nil
}
