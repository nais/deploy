// package deployServer provides message streams between hookd and deployd

package deployserver

import (
	"context"
	"fmt"
	"time"

	"github.com/navikt/deployment/pkg/pb"
	database_mapper "github.com/navikt/deployment/pkg/hookd/database/mapper"
	"github.com/navikt/deployment/pkg/hookd/github"
	"github.com/navikt/deployment/pkg/hookd/metrics"
	log "github.com/sirupsen/logrus"
)

var (
	requestTimeout  = time.Second * 5
	errNoRepository = fmt.Errorf("no repository specified")
)

func (s *deployServer) githubLoop() {
	for {
		select {
		case request := <-s.requests:
			logger := log.WithFields(request.LogFields())
			err := s.createGithubDeployment(request)
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

		case status := <-s.statuses:
			logger := log.WithFields(status.LogFields())
			err := s.createGithubDeploymentStatus(status)
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

func (s *deployServer) createGithubDeployment(request pb.DeploymentRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	repo := request.GetDeployment().GetRepository()
	if !repo.Valid() {
		return errNoRepository
	}

	err := s.githubClient.TeamAllowed(ctx, repo.GetOwner(), repo.GetName(), request.GetPayloadSpec().GetTeam())
	if err != nil {
		return err
	}

	ghdeploy, err := s.githubClient.CreateDeployment(ctx, request)
	if err != nil {
		return fmt.Errorf("create GitHub deployment: %s", err)
	}

	deploy, err := s.db.Deployment(ctx, request.GetDeliveryID())
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

	err = s.db.WriteDeployment(ctx, *deploy)
	if err != nil {
		return fmt.Errorf("write GitHub deployment ID to database: %s", err)
	}

	return nil
}

func (s *deployServer) createGithubDeploymentStatus(status pb.DeploymentStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	deploy, err := s.db.Deployment(ctx, status.GetDeliveryID())
	if err != nil {
		return fmt.Errorf("get deployment from database: %s", err)
	}

	if deploy.GitHubID == nil {
		return fmt.Errorf("GitHub deployment ID not recorded in database")
	}

	status.Deployment.DeploymentID = int64(*deploy.GitHubID)
	_, err = s.githubClient.CreateDeploymentStatus(ctx, status)
	if err != nil {
		return fmt.Errorf("create GitHub deployment status: %s", err)
	}

	return nil
}

func (s *deployServer) SendDeploymentRequest(ctx context.Context, request pb.DeploymentRequest) error {
	err := s.clusterOnline(request.Cluster)
	if err != nil {
		return err
	}
	err = s.streams[request.Cluster].Send(&request)
	if err != nil {
		return err
	}

	log.WithFields(request.LogFields()).Infof("Sent deployment request")

	s.requests <- request

	return nil
}

func (s *deployServer) clusterOnline(clusterName string) error {
	_, ok := s.streams[clusterName]
	if !ok {
		return fmt.Errorf("cluster '%s' is offline", clusterName)
	}
	return nil
}

func (s *deployServer) HandleDeploymentStatus(ctx context.Context, status pb.DeploymentStatus) error {
	dbStatus := database_mapper.DeploymentStatus(status)
	err := s.db.WriteDeploymentStatus(ctx, dbStatus)
	if err != nil {
		return fmt.Errorf("write to database: %s", err)
	}

	metrics.UpdateQueue(status)

	log.WithFields(status.LogFields()).Infof("Saved deployment status")

	s.statuses <- status

	return nil
}
