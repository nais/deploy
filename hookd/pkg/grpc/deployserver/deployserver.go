package deployserver

import (
	"context"
	"fmt"

	"github.com/navikt/deployment/common/pkg/deployment"
)

const channelSize = 1000

type DeployServer interface {
	deployment.DeployServer
	Queue(request *deployment.DeploymentRequest)
}

type deployServer struct {
	channels map[string]chan *deployment.DeploymentRequest
}

func New(clusters []string) DeployServer {
	server := &deployServer{
		channels: make(map[string]chan *deployment.DeploymentRequest),
	}
	for _, cluster := range clusters {
		server.channels[cluster] = make(chan *deployment.DeploymentRequest, channelSize)
	}
	return server
}

var _ DeployServer = &deployServer{}

func (s *deployServer) Queue(request *deployment.DeploymentRequest) {
	s.channels[request.Cluster] <- request
}

func (s *deployServer) Deployments(deploymentOpts *deployment.GetDeploymentOpts, deploymentsServer deployment.Deploy_DeploymentsServer) error {
	for message := range s.channels[deploymentOpts.Cluster] {
		err := deploymentsServer.Send(message)
		if err != nil {
			return fmt.Errorf("unable to send deployment message: %w", err)
		}
	}
	return fmt.Errorf("channel closed unexpectedly")
}

func (s *deployServer) ReportStatus(context.Context, *deployment.DeploymentStatus) (*deployment.ReportStatusOpts, error) {
	return nil, fmt.Errorf("not implemented")
}

/*
func (m sarama.ConsumerMessage)(bool, error) {
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
*/
