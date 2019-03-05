package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/deployd/pkg/config"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"os"
	"os/signal"
	"time"
)

var cfg = config.DefaultConfig()

type Message struct {
	KafkaMessage sarama.ConsumerMessage
	Request      deployment.DeploymentRequest
	Logger       log.Entry
}

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")

	kafka.SetupFlags(&cfg.Kafka)
}

func matchesCluster(msg Message) error {
	if msg.Request.GetCluster() != cfg.Cluster {
		return fmt.Errorf("message is addressed to cluster %s, not %s", msg.Request.Cluster, cfg.Cluster)
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

func SendDeploymentStatus(status *deployment.DeploymentStatus, producer sarama.SyncProducer, logger log.Entry) error {
	payload, err := proto.Marshal(status)
	if err != nil {
		return fmt.Errorf("while marshalling response Protobuf message: %s", err)
	}

	reply := &sarama.ProducerMessage{
		Topic:     cfg.Kafka.StatusTopic,
		Timestamp: time.Now(),
		Value:     sarama.StringEncoder(payload),
	}

	_, offset, err := producer.SendMessage(reply)
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

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	kafkaLogger, err := logging.New(cfg.Kafka.Verbosity, cfg.LogFormat)
	if err != nil {
		return err
	}

	log.Infof("deployd starting up")
	log.Infof("cluster.................: %s", cfg.Cluster)
	log.Infof("kafka topic for requests: %s", cfg.Kafka.RequestTopic)
	log.Infof("kafka topic for statuses: %s", cfg.Kafka.StatusTopic)
	log.Infof("kafka consumer group....: %s", cfg.Kafka.GroupID)
	log.Infof("kafka brokers...........: %+v", cfg.Kafka.Brokers)

	sarama.Logger = kafkaLogger

	client, err := kafka.NewDualClient(
		cfg.Kafka.Brokers,
		cfg.Kafka.ClientID,
		cfg.Kafka.GroupID,
		cfg.Kafka.RequestTopic,
		cfg.Kafka.StatusTopic,
	)
	if err != nil {
		return fmt.Errorf("while setting up Kafka: %s", err)
	}

	go client.ConsumerLoop()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	for {
		select {
		case m := <-client.RecvQ:
			// Decode Kafka payload into Protobuf with logging metadata
			msg, err := Decode(m)
			if err != nil {
				msg.Logger.Trace(err)
				client.Consumer.MarkOffset(&m, "")
				continue
			}

			msg.Logger.Tracef("incoming request: %s", msg.Request.String())

			// Check if we are the authoritative handler for this message
			if err = matchesCluster(msg); err != nil {
				msg.Logger.Trace(err)
				client.Consumer.MarkOffset(&m, "")
				continue
			}

			err = Preflight(msg)
			if err == nil {
				msg.Logger.Infof("deployment request accepted")
				err = Deploy(msg)
			} else {
				msg.Logger.Warn("deployment request rejected: %s", err)
			}

			var status *deployment.DeploymentStatus
			if err == nil {
				status = successMessage(msg)
			} else {
				status = failureMessage(msg, err)
			}

			err = SendDeploymentStatus(status, client.Producer, msg.Logger)
			if err != nil {
				msg.Logger.Errorf("while transmitting deployment status back to sender: %s")
			}

			client.Consumer.MarkOffset(&m, "")

		case <-signals:
			return nil
		}
	}
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
