package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	"github.com/golang/protobuf/proto"
	"github.com/navikt/deployment/common/pkg/deployment"
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
	flag.StringSliceVar(&cfg.Kafka.Brokers, "kafka-brokers", cfg.Kafka.Brokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.Kafka.Topic, "kafka-topic", cfg.Kafka.Topic, "Kafka topic for deployd communication.")
	flag.StringVar(&cfg.Kafka.ClientID, "kafka-client-id", cfg.Kafka.ClientID, "Kafka client ID.")
	flag.StringVar(&cfg.Kafka.GroupID, "kafka-group-id", cfg.Kafka.GroupID, "Kafka consumer group ID.")
	flag.StringVar(&cfg.Kafka.Verbosity, "kafka-log-verbosity", cfg.Kafka.Verbosity, "Log verbosity for Kafka client.")
}

func consumerLoop(consumer *cluster.Consumer, messages chan<- Message) {
	log.Info("starting up Kafka consumer loop")

	for {
		select {
		case m, op := <-consumer.Messages():
			if !op {
				log.Info("shutting down Kafka consumer loop")
				return
			}

			msg := Message{
				KafkaMessage: *m,
				Logger: *log.WithFields(log.Fields{
					"kafka_offset":    m.Offset,
					"kafka_timestamp": m.Timestamp,
					"kafka_topic":     m.Topic,
				}),
			}

			msg.Logger.Trace("received incoming message")

			err := proto.Unmarshal(m.Value, &msg.Request)
			if err != nil {
				msg.Logger.Error("while decoding Protobuf: %s", err)
				consumer.MarkOffset(m, "")
				continue
			}

			msg.Logger = *msg.Logger.WithField("delivery_id", msg.Request.CorrelationID)

			messages <- msg

		case err := <-consumer.Errors():
			if err != nil {
				log.Errorf("kafka error: %s", err)
			}

		case notif := <-consumer.Notifications():
			log.Warnf("kafka notification: %+v", notif)
		}
	}
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

func messageHandler(msg Message) error {
	msg.Logger.Debug(msg.Request.String())

	err := messageFilter(msg, []MessageFilter{
		meetsDeadline,
		matchesCluster,
	})
	if err != nil {
		return err
	}

	msg.Logger.Infof("deployment request accepted")

	return nil
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
	log.Infof("kafka topic: %s", cfg.Kafka.Topic)
	log.Infof("kafka consumer group: %s", cfg.Kafka.GroupID)
	log.Infof("kafka brokers: %+v", cfg.Kafka.Brokers)

	sarama.Logger = kafkaLogger

	// Instantiate a Kafka client operating in consumer group mode,
	// starting from the oldest unread offset.
	kafkacfg := cluster.NewConfig()
	kafkacfg.ClientID = cfg.Kafka.ClientID
	kafkacfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumer, err := cluster.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID, []string{cfg.Kafka.Topic}, kafkacfg)
	if err != nil {
		return fmt.Errorf("while setting up Kafka consumer: %s", err)
	}

	// Retrieve messages from Kafka in the background
	messages := make(chan Message, 1024)
	go consumerLoop(consumer, messages)

	defer func() {
		close(messages)
		if err := consumer.Close(); err != nil {
			log.Error("unable to shut down Kafka consumer: %s", err)
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	for {
		select {
		case msg := <-messages:
			err := messageHandler(msg)
			if err != nil {
				msg.Logger.Errorf("while handling deployment request: %s", err)
			}
			consumer.MarkOffset(&msg.KafkaMessage, "")

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
