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
}

func NewDualClient(brokers []string, clientID, groupID, consumerTopic, producerTopic string) (*DualClient, error) {
	var err error
	client := &DualClient{}

	// Instantiate a Kafka client operating in consumer group mode,
	// starting from the oldest unread offset.
	consumerCfg := cluster.NewConfig()
	consumerCfg.ClientID = clientID
	consumerCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	client.Consumer, err = cluster.NewConsumer(brokers, groupID, []string{consumerTopic}, consumerCfg)
	if err != nil {
		return nil, fmt.Errorf("while setting up Kafka consumer: %s", err)
	}

	// Instantiate another client, this one in synchronous producer mode.
	client.Producer, err = sarama.NewSyncProducer(brokers, nil)
	if err != nil {
		return nil, fmt.Errorf("while setting up Kafka producer: %s", err)
	}

	client.RecvQ = make(chan sarama.ConsumerMessage, 1024)

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

