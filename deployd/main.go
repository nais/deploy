package main

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/deployd/pkg/config"
	"github.com/navikt/deployment/deployd/pkg/deployd"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
	"os"
	"os/signal"
)

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")

	kafka.SetupFlags(&cfg.Kafka)
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

	var m sarama.ConsumerMessage

	for {
		select {
		case m = <-client.RecvQ:
			if msg, err := deployd.Handle(client, m, cfg.Cluster); err != nil {
				msg.Logger.Errorf("while transmitting deployment status back to sender: %s")
			}


		case <-signals:
			return nil
		}

		client.Consumer.MarkOffset(&m, "")
	}
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
