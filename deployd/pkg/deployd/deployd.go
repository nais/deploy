package deployd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	log "github.com/sirupsen/logrus"
	"io"
	"os/exec"
	"time"
)

type Message struct {
	KafkaMessage sarama.ConsumerMessage
	Request      deployment.DeploymentRequest
	Logger       log.Entry
}

type Payload struct {
	Version    [3]int
	Team       string
	Kubernetes struct {
		Resources []json.RawMessage
	}
}

func kubectlApply(resource []byte, stdout, stderr io.Writer) error {
	cmd := exec.Command("kubectl", "apply", "--filename=-")
	r := bytes.NewReader(resource)
	cmd.Stdin = r
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
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
	payload, err := deployment.WrapMessage(status, client.SignatureKey)
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

	return nil
}

// Deploy something to Kubernetes
func Deploy(msg Message) error {
	payload := Payload{}
	err := json.Unmarshal(msg.Request.Payload, &payload)
	if err != nil {
		return fmt.Errorf("while decoding payload: %s", err)
	}

	numResources := len(payload.Kubernetes.Resources)
	if numResources == 0 {
		return fmt.Errorf("no resources to deploy")
	}

	if len(payload.Team) == 0 {
		return fmt.Errorf("team not specified in deployment payload")
	}

	msg.Logger.Infof("deploying %d resources to Kubernetes on behalf of team %s", numResources, payload.Team)

	for index, r := range payload.Kubernetes.Resources {
		outbuf := make([]byte, 0)
		buf := bytes.NewBuffer(outbuf)
		err := kubectlApply(r, buf, buf)
		msg.Logger.Tracef("resource %d: %s", index+1, buf.String())
		if err != nil {
			return fmt.Errorf("while deploying resource %d: %s", index+1, err)
		}
	}

	return nil
}

func Decode(m sarama.ConsumerMessage, key string) (Message, error) {
	msg := Message{
		KafkaMessage: m,
		Logger:       kafka.ConsumerMessageLogger(&m),
	}

	if err := deployment.UnwrapMessage(m.Value, key, &msg.Request); err != nil {
		return msg, err
	}

	msg.Logger = *msg.Logger.WithField("delivery_id", msg.Request.GetDeliveryID())

	return msg, nil
}

func Handle(client *kafka.DualClient, m sarama.ConsumerMessage, cluster string) (Message, error) {
	// Decode Kafka payload into Protobuf with logging metadata
	msg, err := Decode(m, client.SignatureKey)
	if err != nil {
		msg.Logger.Errorf("unable to process message: %s", err)
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
		if err != nil {
			msg.Logger.Errorf("deployment failed: %s", err)
		}
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
