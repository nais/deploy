// package broker provides message switching between hookd and Kafka

package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	gh "github.com/google/go-github/v27/github"
	"github.com/navikt/deployment/common/pkg/deployment"
	api_v1 "github.com/navikt/deployment/hookd/pkg/api/v1"
	"github.com/navikt/deployment/hookd/pkg/database"
	database_mapper "github.com/navikt/deployment/hookd/pkg/database/mapper"
	"github.com/navikt/deployment/hookd/pkg/github"
	log "github.com/sirupsen/logrus"
)

var (
	requestTimeout = time.Second * 5
)

type broker struct {
	db           database.DeploymentStore
	producer     sarama.SyncProducer
	serializer   Serializer
	githubClient github.Client
	requests     chan deployment.DeploymentRequest
	statuses     chan deployment.DeploymentStatus
}

type Broker interface {
	SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error
}

func New(db database.DeploymentStore, producer sarama.SyncProducer, serializer Serializer, githubClient github.Client) Broker {
	b := &broker{
		db:           db,
		producer:     producer,
		serializer:   serializer,
		githubClient: githubClient,
		requests:     make(chan deployment.DeploymentRequest, 4096),
		statuses:     make(chan deployment.DeploymentStatus, 4096),
	}
	go b.githubLoop()
	return b
}

func (b *broker) githubLoop() {
	for {
		select {
		case request := <-b.requests:
			logger := log.WithFields(request.LogFields())
			err := b.createGithubDeployment(request)
			if err != nil {
				logger.Errorf("Unable to sync deployment to GitHub: %s", err)
				continue
			}
			logger.Tracef("Synchronized deployment to GitHub")

		case status := <-b.statuses:
			logger := log.WithFields(status.LogFields())
			err := b.createGithubDeploymentStatus(status)
			if err != nil {
				logger.Errorf("Unable to sync deployment status to GitHub: %s", err)
				continue
			}
			logger.Tracef("Synchronized deployment status to GitHub")
		}
	}
}

func (b *broker) createGithubDeployment(request deployment.DeploymentRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	repo := request.GetDeployment().GetRepository()

	err := b.githubClient.TeamAllowed(ctx, repo.GetOwner(), repo.GetName(), request.GetPayloadSpec().GetTeam())
	if err != nil {
		return err
	}

	payload := github.DeploymentRequest(request)
	ghdeploy, err := b.githubClient.CreateDeployment(ctx, repo.GetOwner(), repo.GetName(), &payload)
	if err != nil {
		return fmt.Errorf("create GitHub deployment: %s", err)
	}

	deploy, err := b.db.Deployment(ctx, request.GetDeliveryID())
	if err != nil {
		return fmt.Errorf("get deployment from database: %s", err)
	}

	id := int(ghdeploy.GetID())
	if id == 0 {
		return fmt.Errorf("GitHub deployment ID is zero")
	}
	fullName := repo.FullName()

	deploy.GitHubID = &id
	deploy.GitHubRepository = &fullName

	err = b.db.WriteDeployment(ctx, *deploy)
	if err != nil {
		return fmt.Errorf("write GitHub deployment ID to database: %s", err)
	}

	return nil
}

func (b *broker) createGithubDeploymentStatus(status deployment.DeploymentStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	deploy, err := b.db.Deployment(ctx, status.GetDeliveryID())
	if err != nil {
		return fmt.Errorf("get deployment from database: %s", err)
	}

	if deploy.GitHubID == nil {
		return fmt.Errorf("GitHub deployment ID not recorded in database")
	}

	status.Deployment.DeploymentID = int64(*deploy.GitHubID)
	_, err = b.githubClient.CreateDeploymentStatus(ctx, &status)
	if err != nil {
		return fmt.Errorf("create GitHub deployment status: %s", err)
	}

	return nil
}

func (b *broker) SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error {
	msg, err := b.serializer.Marshal(deployment)
	if err != nil {
		return err
	}

	_, _, err = b.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("publish message to Kafka: %s", err)
	}

	log.WithFields(deployment.LogFields()).Infof("Sent deployment request")

	b.requests <- deployment

	return nil
}

func (b *broker) HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error {
	dbStatus := database_mapper.DeploymentStatus(status)
	err := b.db.WriteDeploymentStatus(ctx, dbStatus)
	if err != nil {
		return fmt.Errorf("write to database: %s", err)
	}

	log.WithFields(status.LogFields()).Infof("Saved deployment status")

	b.statuses <- status

	return nil
}
