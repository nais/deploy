// package broker provides message switching between hookd and Kafka

package broker

import (
	"context"
	"fmt"
	"time"

	"github.com/navikt/deployment/hookd/pkg/grpc/deployserver"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/navikt/deployment/hookd/pkg/database"
	database_mapper "github.com/navikt/deployment/hookd/pkg/database/mapper"
	"github.com/navikt/deployment/hookd/pkg/github"
	"github.com/navikt/deployment/hookd/pkg/metrics"
	log "github.com/sirupsen/logrus"
)

var (
	requestTimeout  = time.Second * 5
	errNoRepository = fmt.Errorf("no repository specified")
)

type broker struct {
	db           database.DeploymentStore
	grpc         deployserver.DeployServer
	githubClient github.Client
	requests     chan deployment.DeploymentRequest
	statuses     chan deployment.DeploymentStatus
}

type Broker interface {
	SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error
	HandleDeploymentStatus(ctx context.Context, status deployment.DeploymentStatus) error
}

func New(db database.DeploymentStore, grpc deployserver.DeployServer, githubClient github.Client) Broker {
	b := &broker{
		db:           db,
		grpc:         grpc,
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
			switch err {
			case github.ErrTeamNotExist:
				logger.Errorf(
					"Not syncing deployment to GitHub: team %s does not exist on GitHub",
					request.GetPayloadSpec().GetTeam(),
				)
			case github.ErrTeamNoAccess:
				logger.Errorf(
					"Not syncing deployment to GitHub: team %s does not have admin rights to repository %s",
					request.GetPayloadSpec().GetTeam(),
					request.GetDeployment().GetRepository().FullName(),
				)
			case nil:
				logger.Tracef("Synchronized deployment to GitHub")
			default:
				logger.Errorf("Unable to sync deployment to GitHub: %s", err)
			}

		case status := <-b.statuses:
			logger := log.WithFields(status.LogFields())
			err := b.createGithubDeploymentStatus(status)
			switch err {
			case errNoRepository:
				logger.Tracef("Not syncing deployment to GitHub: %s", err)
			case nil:
				logger.Tracef("Synchronized deployment status to GitHub")
			default:
				logger.Errorf("Unable to sync deployment status to GitHub: %s", err)
			}
		}
	}
}

func (b *broker) createGithubDeployment(request deployment.DeploymentRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	repo := request.GetDeployment().GetRepository()
	if !repo.Valid() {
		return errNoRepository
	}

	err := b.githubClient.TeamAllowed(ctx, repo.GetOwner(), repo.GetName(), request.GetPayloadSpec().GetTeam())
	if err != nil {
		return err
	}

	ghdeploy, err := b.githubClient.CreateDeployment(ctx, request)
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
	_, err = b.githubClient.CreateDeploymentStatus(ctx, status)
	if err != nil {
		return fmt.Errorf("create GitHub deployment status: %s", err)
	}

	return nil
}

func (b *broker) SendDeploymentRequest(ctx context.Context, deployment deployment.DeploymentRequest) error {
	b.grpc.Queue(&deployment)

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

	metrics.UpdateQueue(status)

	log.WithFields(status.LogFields()).Infof("Saved deployment status")

	b.statuses <- status

	return nil
}
