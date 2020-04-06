// package broker provides message switching between hookd and Kafka

package broker

import (
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/database"
	database_mapper "github.com/navikt/deployment/hookd/pkg/database/mapper"
	log "github.com/sirupsen/logrus"
)

type broker struct {
	db         database.DeploymentStore
	producer   sarama.SyncProducer
	serializer Serializer
}

type Broker interface {
	SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error
}

func New(db database.DeploymentStore, producer sarama.SyncProducer, serializer Serializer) Broker {
	return &broker{
		db:         db,
		producer:   producer,
		serializer: serializer,
	}
}

func (b *broker) SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error {
	msg, err := b.serializer.Marshal(deployment)
	if err != nil {
		return err
	}

	_, _, err = b.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("publish message to Kafka: %s", err)
	}

	log.WithFields(deployment.LogFields()).Infof("Sent deployment request")

	return nil
}

func (b *broker) HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error {
	dbStatus := database_mapper.DeploymentStatus(status)
	err := b.db.WriteDeploymentStatus(ctx, dbStatus)
	if err != nil {
		return fmt.Errorf("write to database: %s", err)
	}

	log.WithFields(status.LogFields()).Infof("Saved deployment status")

	return nil
}
