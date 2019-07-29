package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/common/pkg/logging"
	"github.com/navikt/deployment/deployd/pkg/config"
	"github.com/navikt/deployment/deployd/pkg/deployd"
	"github.com/navikt/deployment/deployd/pkg/kubeclient"
	"github.com/navikt/deployment/deployd/pkg/metrics"
	log "github.com/sirupsen/logrus"
	flag "github.com/spf13/pflag"
)

var cfg = config.DefaultConfig()

func init() {
	flag.StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format, either 'json' or 'text'.")
	flag.StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Logging verbosity level.")
	flag.StringVar(&cfg.Cluster, "cluster", cfg.Cluster, "Apply changes only within this cluster.")
	flag.StringVar(&cfg.MetricsListenAddr, "metrics-listen-addr", cfg.MetricsListenAddr, "Serve metrics on this address.")
	flag.StringVar(&cfg.MetricsPath, "metrics-path", cfg.MetricsPath, "Serve metrics on this endpoint.")

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

	sarama.Logger = kafkaLogger

	log.Infof("deployd starting up")
	log.Infof("cluster.................: %s", cfg.Cluster)

	kube, err := kubeclient.New()
	if err != nil {
		return fmt.Errorf("cannot configure Kubernetes client: %s", err)
	}
	log.Infof("kubernetes..............: %s", kube.Config.Host)

	log.Infof("kafka topic for requests: %s", cfg.Kafka.RequestTopic)
	log.Infof("kafka topic for statuses: %s", cfg.Kafka.StatusTopic)
	log.Infof("kafka consumer group....: %s", cfg.Kafka.GroupID)
	log.Infof("kafka brokers...........: %+v", cfg.Kafka.Brokers)

	client, err := kafka.NewDualClient(
		cfg.Kafka,
		cfg.Kafka.RequestTopic,
		cfg.Kafka.StatusTopic,
	)
	if err != nil {
		return fmt.Errorf("while setting up Kafka: %s", err)
	}

	go client.ConsumerLoop()

	statusChan := make(chan *deployment.DeploymentStatus, 1024)

	metricsServer := http.NewServeMux()
	metricsServer.Handle(cfg.MetricsPath, metrics.Handler())
	log.Infof("Serving metrics on %s endpoint %s", cfg.MetricsListenAddr, cfg.MetricsPath)
	go http.ListenAndServe(cfg.MetricsListenAddr, metricsServer)

	// Trap SIGINT to trigger a shutdown.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt)

	var m sarama.ConsumerMessage

	for {
	SEL:
		select {
		case m = <-client.RecvQ:
			logger := kafka.ConsumerMessageLogger(&m)

			// Check the validity and authenticity of the message.
			deployd.Run(&logger, m.Value, client.SignatureKey, cfg.Cluster, kube, statusChan)

		case status := <-statusChan:
			logger := log.WithFields(status.LogFields())
			switch {
			case status == nil:
				metrics.DeployIgnored.Inc()
				break SEL
			case status.GetState() == deployment.GithubDeploymentState_error:
				fallthrough
			case status.GetState() == deployment.GithubDeploymentState_failure:
				metrics.DeployFailed.Inc()
				logger.Errorf(status.GetDescription())
			default:
				metrics.DeploySuccessful.Inc()
				logger.Infof(status.GetDescription())
			}

			err = SendDeploymentStatus(status, client)
			if err != nil {
				logger.Errorf("While reporting deployment status: %s", err)
			}

		case <-signals:
			return nil
		}

		client.Consumer.MarkOffset(&m, "")
	}
}

func SendDeploymentStatus(status *deployment.DeploymentStatus, client *kafka.DualClient) error {
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

	logger := log.WithFields(status.LogFields())
	logger.WithFields(log.Fields{
		"kafka_offset":    offset,
		"kafka_timestamp": reply.Timestamp,
		"kafka_topic":     reply.Topic,
	}).Infof("Deployment response sent successfully")

	return nil
}

func main() {
	err := run()
	if err != nil {
		log.Errorf("Fatal error: %s", err)
		os.Exit(1)
	}
}
