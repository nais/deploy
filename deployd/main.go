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

type Handler struct {
	Message  Message
	Producer sarama.SyncProducer
}

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")

	kafka.SetupFlags(&cfg.Kafka)
}

type MessageFilter func(Message) error

func matchesCluster(msg Message) error {
	if msg.Request.GetCluster() != cfg.Cluster {
		return fmt.Errorf("request is for cluster %s, not %s", msg.Request.Cluster, cfg.Cluster)
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

func messageFilter(msg Message, filters []MessageFilter) error {
	for _, f := range filters {
		if err := f(msg); err != nil {
			return err
		}
	}
	return nil
}

func (h *Handler) Handle() error {
	h.Message.Logger.Debug(h.Message.Request.String())

	err := messageFilter(h.Message, []MessageFilter{
		meetsDeadline,
		matchesCluster,
	})
	if err != nil {
		return err
	}

	h.Message.Logger.Infof("deployment request accepted")

	return nil
}

func (h *Handler) SendDeploymentStatus(status *deployment.DeploymentStatus) error {
	payload, err := proto.Marshal(status)
	if err != nil {
		return fmt.Errorf("while marshalling response Protobuf message: %s", err)
	}

	reply := &sarama.ProducerMessage{
		Topic:     cfg.Kafka.StatusTopic,
		Timestamp: time.Now(),
		Value:     sarama.StringEncoder(payload),
	}

	_, offset, err := h.Producer.SendMessage(reply)
	if err != nil {
		return fmt.Errorf("while sending reply over Kafka: %s", err)
	}

	h.Message.Logger.WithFields(log.Fields{
		"kafka_offset":    offset,
		"kafka_topic":     reply.Topic,
		"kafka_timestamp": reply.Timestamp,
	}).Infof("deployment response sent successfully")

	return nil
}

func (h *Handler) SendFailure(handlerError error) error {
	status := &deployment.DeploymentStatus{
		Deployment:  h.Message.Request.Deployment,
		Description: fmt.Sprintf("deployment failed: %s", handlerError),
		State:       deployment.GithubDeploymentState_failure,
		DeliveryID:  h.Message.Request.GetDeliveryID(),
	}
	return h.SendDeploymentStatus(status)
}

func (h *Handler) SendSuccess() error {
	status := &deployment.DeploymentStatus{
		Deployment:  h.Message.Request.Deployment,
		Description: fmt.Sprintf("deployment succeeded"),
		State:       deployment.GithubDeploymentState_success,
		DeliveryID:  h.Message.Request.GetDeliveryID(),
	}
	return h.SendDeploymentStatus(status)
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
			msg := Message{
				KafkaMessage: m,
				Logger: *log.WithFields(log.Fields{
					"kafka_offset":    m.Offset,
					"kafka_timestamp": m.Timestamp,
					"kafka_topic":     m.Topic,
				}),
			}

			msg.Logger.Trace("received incoming message")

			err := proto.Unmarshal(m.Value, &msg.Request)
			if err != nil {
				msg.Logger.Errorf("while decoding Protobuf: %s", err)
				client.Consumer.MarkOffset(&m, "")
				continue
			}

			msg.Logger = *msg.Logger.WithField("delivery_id", msg.Request.GetDeliveryID())

			handler := Handler{
				Message:  msg,
				Producer: client.Producer,
			}

			err = handler.Handle()

			if err == nil {
				err = handler.SendSuccess()
			} else {
				handler.Message.Logger.Errorf("while deploying: %s", err)
				err = handler.SendFailure(err)
			}

			if err != nil {
				handler.Message.Logger.Errorf("while transmitting deployment status back to sender: %s")
			}

			client.Consumer.MarkOffset(&msg.KafkaMessage, "")

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
