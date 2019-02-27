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
)

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringSliceVar(&cfg.Kafka.Brokers, "kafka-brokers", cfg.Kafka.Brokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.Kafka.Topic, "kafka-topic", cfg.Kafka.Topic, "Kafka topic for deployd communication.")
	flag.StringVar(&cfg.Kafka.ClientID, "kafka-client-id", cfg.Kafka.ClientID, "Kafka client ID.")
	flag.StringVar(&cfg.Kafka.GroupID, "kafka-group-id", cfg.Kafka.GroupID, "Kafka consumer group ID.")
}

func run() error {
	flag.Parse()

	sarama.Logger = log.New()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	kafkacfg := cluster.NewConfig()
	kafkacfg.ClientID = cfg.Kafka.ClientID
	kafkacfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumer, err := cluster.NewConsumer(cfg.Kafka.Brokers, cfg.Kafka.GroupID, []string{cfg.Kafka.Topic}, kafkacfg)
	if err != nil {
		return err
	}

	defer func() {
		if err := consumer.Close(); err != nil {
			log.Fatalln(err)
		}
	}()

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	consumed := 0
ConsumerLoop:
	for {
		select {
		case msg := <-consumer.Messages():
			log.Printf("Consumed message offset %d\n", msg.Offset)
			consumed++

			deploymentRequest := &deployment.DeploymentRequest{}
			err := proto.Unmarshal(msg.Value, deploymentRequest)
			if err != nil {
				log.Error(fmt.Errorf("while decoding Protobuf: %s", err))
				consumer.MarkOffset(msg, "")
				continue
			}
			fmt.Println(deploymentRequest.String())
			consumer.MarkOffset(msg, "")

		case err = <-consumer.Errors():
			log.Errorf("kafka error: %s", err)

		case notif := <-consumer.Notifications():
			log.Warnf("kafka notification: %+v", notif)

		case <-signals:
			break ConsumerLoop
		}
	}

	log.Printf("Consumed: %d\n", consumed)

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
