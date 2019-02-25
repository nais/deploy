package main

import (
	"fmt"
	"github.com/Shopify/sarama"
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
	flag.StringSliceVar(&cfg.KafkaBrokers, "kafka-brokers", cfg.KafkaBrokers, "Comma-separated list of Kafka brokers, HOST:PORT.")
	flag.StringVar(&cfg.KafkaTopic, "kafka-topic", cfg.KafkaTopic, "Kafka topic for deployd communication.")
}

func run() error {
	flag.Parse()

	if err := logging.Setup(cfg.LogLevel, cfg.LogFormat); err != nil {
		return err
	}

	consumer, err := sarama.NewConsumer(cfg.KafkaBrokers, nil)
	if err != nil {
		return err
	}
	partitionConsumer, err := consumer.ConsumePartition(cfg.KafkaTopic, 0, sarama.OffsetNewest)
	if err != nil {
		return err
	}

	defer func() {
		if err := partitionConsumer.Close(); err != nil {
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
		case msg := <-partitionConsumer.Messages():
			log.Printf("Consumed message offset %d\n", msg.Offset)
			consumed++

			deploymentRequest := &deployment.DeploymentRequest{}
			err := proto.Unmarshal(msg.Value, deploymentRequest)
			if err != nil {
				log.Error(err)
				continue
			}
			fmt.Println(deploymentRequest.String())

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
