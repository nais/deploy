package deployd

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	log "github.com/sirupsen/logrus"
	"time"
)

type Message struct {
	KafkaMessage sarama.ConsumerMessage
	Request      deployment.DeploymentRequest
	Logger       log.Entry
}

func matchesCluster(msg Message, cluster string) error {
	if msg.Request.GetCluster() != cluster {
		return fmt.Errorf("message is addressed to cluster %s, not %s", msg.Request.Cluster, cluster)
	}
	return nil
}

func meetsDeadline(msg Message) error {
	deadline := time.Unix(msg.Request.GetDeadline(), 0)
	late := time.Since(deadline)
	if late > 0 {
		return fmt.Errorf("deadline exceeded by %s", late.String())
	}
	return nil
}

func aclCheck(msg Message) error {
	return nil
}

func failureMessage(msg Message, handlerError error) *deployment.DeploymentStatus {
	return &deployment.DeploymentStatus{
		Deployment:  msg.Request.Deployment,
		Description: fmt.Sprintf("deployment failed: %s", handlerError),
		State:       deployment.GithubDeploymentState_failure,
		DeliveryID:  msg.Request.GetDeliveryID(),
	}
}

func successMessage(msg Message) *deployment.DeploymentStatus {
	return &deployment.DeploymentStatus{
		Deployment:  msg.Request.Deployment,
		Description: fmt.Sprintf("deployment succeeded"),
		State:       deployment.GithubDeploymentState_success,
		DeliveryID:  msg.Request.GetDeliveryID(),
	}
}

func SendDeploymentStatus(status *deployment.DeploymentStatus, client *kafka.DualClient, logger log.Entry) error {
	payload, err := proto.Marshal(status)
	if err != nil {
		return fmt.Errorf("while marshalling response Protobuf message: %s", err)
	}

	reply := &sarama.ProducerMessage{
		Topic:     client.ProducerTopic,
		Timestamp: time.Now(),
		Value:     sarama.StringEncoder(payload),
	}

	_, offset, err := client.Producer.SendMessage(reply)
	if err != nil {
		return fmt.Errorf("while sending reply over Kafka: %s", err)
	}

	logger.WithFields(log.Fields{
		"kafka_offset":    offset,
		"kafka_timestamp": reply.Timestamp,
		"kafka_topic":     reply.Topic,
	}).Infof("deployment response sent successfully")

	return nil
}

// Check conditions before deployment
func Preflight(msg Message) error {
	if err := meetsDeadline(msg); err != nil {
		return err
	}

	if err := aclCheck(msg); err != nil {
		return err
	}

	return nil
}

// Deploy something to Kubernetes
func Deploy(msg Message) error {
	msg.Logger.Infof("no-op: deploying to Kubernetes!")
	return nil
}

func Decode(m sarama.ConsumerMessage) (Message, error) {
	msg := Message{
		KafkaMessage: m,
		Logger:       kafka.ConsumerMessageLogger(&m),
	}

	if err := proto.Unmarshal(m.Value, &msg.Request); err != nil {
		return msg, fmt.Errorf("while decoding Protobuf: %s", err)
	}

	msg.Logger = *msg.Logger.WithField("delivery_id", msg.Request.GetDeliveryID())

	return msg, nil
}

func Handle(client *kafka.DualClient, m sarama.ConsumerMessage, cluster string) (Message, error) {
	// Decode Kafka payload into Protobuf with logging metadata
	msg, err := Decode(m)
	if err != nil {
		msg.Logger.Trace(err)
		return msg, nil
	}

	msg.Logger.Tracef("incoming request: %s", msg.Request.String())

	// Check if we are the authoritative handler for this message
	if err = matchesCluster(msg, cluster); err != nil {
		msg.Logger.Trace(err)
		client.Consumer.MarkOffset(&m, "")
		return msg, nil
	}

	err = Preflight(msg)
	if err == nil {
		msg.Logger.Infof("deployment request accepted")
		err = Deploy(msg)
	} else {
		msg.Logger.Warnf("deployment request rejected: %s", err)
	}

	var status *deployment.DeploymentStatus
	if err == nil {
		status = successMessage(msg)
	} else {
		status = failureMessage(msg, err)
	}

	return msg, SendDeploymentStatus(status, client, msg.Logger)
}
