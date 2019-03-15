package kafka

import (
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/bsm/sarama-cluster"
	log "github.com/sirupsen/logrus"
)

type DualClient struct {
	RecvQ    chan sarama.ConsumerMessage
	Consumer *cluster.Consumer
	Producer sarama.SyncProducer
	ProducerTopic string
}

func NewDualClient(cfg Config, consumerTopic, producerTopic string) (*DualClient, error) {
	var err error
	client := &DualClient{}

	// Instantiate a Kafka client operating in consumer group mode,
	// starting from the oldest unread offset.
	consumerCfg := cluster.NewConfig()
	consumerCfg.ClientID = fmt.Sprintf("%s-consumer", cfg.ClientID)
	consumerCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	consumerCfg.Net.SASL.Enable = cfg.SASL.Enabled
	consumerCfg.Net.SASL.User = cfg.SASL.Username
	consumerCfg.Net.SASL.Password = cfg.SASL.Password
	client.Consumer, err = cluster.NewConsumer(cfg.Brokers, cfg.GroupID, []string{consumerTopic}, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("while setting up Kafka consumer: %s", err)
	}

	// Instantiate another client, this one in synchronous producer mode.
	producerCfg := sarama.NewConfig()
	producerCfg.ClientID = fmt.Sprintf("%s-producer", cfg.ClientID)
	producerCfg.Net.SASL = consumerCfg.Net.SASL
	producerCfg.Producer.Return.Successes = true
	client.Producer, err = sarama.NewSyncProducer(cfg.Brokers, producerCfg)
	if err != nil {
		return nil, fmt.Errorf("while setting up Kafka producer: %s", err)
	}

	client.RecvQ = make(chan sarama.ConsumerMessage, 1024)
	client.ProducerTopic = producerTopic

	return client, nil
}

func (client *DualClient) ConsumerLoop() {
	log.Info("starting up Kafka consumer loop")

	defer func() {
		close(client.RecvQ)
		if err := client.Consumer.Close(); err != nil {
			log.Error("unable to shut down Kafka consumer: %s", err)
		}
	}()

	for {
		select {
		case m, op := <-client.Consumer.Messages():
			if !op {
				log.Info("shutting down Kafka consumer loop")
				return
			}

			log.Tracef("Kafka consumer received message: %+v", m)

			client.RecvQ <- *m

		case err := <-client.Consumer.Errors():
			if err != nil {
				log.Errorf("kafka consumer error: %s", err)
			}

		case notif := <-client.Consumer.Notifications():
			log.Warnf("kafka consumer notification: %+v", notif)
		}
	}
}

func ConsumerMessageLogger(msg *sarama.ConsumerMessage) log.Entry {
	return *log.WithFields(log.Fields{
		"kafka_offset":    msg.Offset,
		"kafka_timestamp": msg.Timestamp,
		"kafka_topic":     msg.Topic,
	})
}

