package deployserver

import (
	"context"
	"fmt"
	"github.com/Shopify/sarama"
	"github.com/navikt/deployment/common/pkg/kafka"
	"github.com/navikt/deployment/hookd/pkg/database"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
)

type DeployServer struct {
}

var _ deployment.DeployServer = &DeployServer{}

func (s *DeployServer) Deployments(*deployment.GetDeploymentOpts, deployment.Deploy_DeploymentsServer) error {
	return fmt.Errorf("not implemented")
}

func (s *DeployServer) ReportStatus(context.Context, *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, fmt.Errorf("not implemented")
}

func(m sarama.ConsumerMessage) (bool, error) {
	retry := false
	status, err := serializer.Unmarshal(m)
	if err != nil {
		return retry, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), retryInterval)
	defer cancel()
	err = sideBrok.HandleDeploymentStatus(ctx, *status)

	switch {
	default:
		retry = true
	case err == nil:
	case database.IsErrForeignKeyViolation(err):
	}

	return retry, err
}

// Loop through incoming deployment status messages from deployd and commit them to the database.
func temp() {
	for {
		select {
		case m := <-kafkaClient.RecvQ:
			var err error
			retry := true
			logger := kafka.ConsumerMessageLogger(&m)

			metrics.KafkaQueueSize.Set(float64(len(kafkaClient.RecvQ)))

			for retry {
				retry, err = handleKafkaStatus(m)
				if err != nil && retry {
					logger.Errorf("process deployment status: %s", err)
					time.Sleep(retryInterval)
				}
			}

			kafkaClient.Consumer.MarkOffset(&m, "")
			if err != nil {
				logger.Errorf("discard deployment status: %s", err)
			}

		case <-signals:
			return nil
		}
	}

}
