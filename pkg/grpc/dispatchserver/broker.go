// package deployServer provides message streams between hookd and deployd

package dispatchserver

import (
	"context"
	"fmt"
	"time"

	database_mapper "github.com/navikt/deployment/pkg/hookd/database/mapper"
	"github.com/navikt/deployment/pkg/hookd/github"
	"github.com/navikt/deployment/pkg/hookd/metrics"
	"github.com/navikt/deployment/pkg/pb"
	log "github.com/sirupsen/logrus"
)

var (
	requestTimeout  = time.Second * 5
	errNoRepository = fmt.Errorf("no repository specified")
)

func (s *dispatchServer) githubLoop() {
	for {
		select {
		case request := <-s.requests:
			logger := log.WithFields(request.LogFields())
			err := s.createGithubDeployment(request)
			switch err {
			case github.ErrTeamNotExist:
				logger.Errorf(
					"Not syncing deployment to GitHub: team %s does not exist on GitHub",
					request.GetTeam(),
				)
			case github.ErrTeamNoAccess:
				logger.Errorf(
					"Not syncing deployment to GitHub: team %s does not have admin rights to repository %s",
					request.GetTeam(),
					request.GetRepository().FullName(),
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

func (s *dispatchServer) createGithubDeployment(request *pb.DeploymentRequest) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	repo := request.GetRepository()
	if !repo.Valid() {
		return errNoRepository
	}

	err := s.githubClient.TeamAllowed(ctx, repo.GetOwner(), repo.GetName(), request.GetTeam())
	if err != nil {
		return err
	}

	ghdeploy, err := s.githubClient.CreateDeployment(ctx, request)
	if err != nil {
		return fmt.Errorf("create GitHub deployment: %s", err)
	}

	deploy, err := s.db.Deployment(ctx, request.GetID())
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

func (s *dispatchServer) createGithubDeploymentStatus(status *pb.DeploymentStatus) error {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout)
	defer cancel()

	deploy, err := s.db.Deployment(ctx, status.GetRequest().GetID())
	if err != nil {
		return fmt.Errorf("get deployment from database: %s", err)
	}

	if deploy.GitHubID == nil {
		return fmt.Errorf("GitHub deployment ID not recorded in database")
	}

	deploymentID := int64(*deploy.GitHubID)
	_, err = s.githubClient.CreateDeploymentStatus(ctx, status, deploymentID)
	if err != nil {
		return fmt.Errorf("create GitHub deployment status: %s", err)
	}

	return nil
}

func (s *dispatchServer) SendDeploymentRequest(ctx context.Context, request *pb.DeploymentRequest) error {
	err := s.clusterOnline(request.Cluster)
	if err != nil {
		return err
	}
	err = s.dispatchStreams[request.Cluster].Send(request)
	if err != nil {
		return err
	}

	log.WithFields(request.LogFields()).Infof("Sent deployment request")

	s.requests <- request

	return nil
}

func (s *dispatchServer) clusterOnline(clusterName string) error {
	_, ok := s.dispatchStreams[clusterName]
	if !ok {
		return fmt.Errorf("cluster '%s' is offline", clusterName)
	}
	return nil
}

func (s *dispatchServer) HandleDeploymentStatus(ctx context.Context, status *pb.DeploymentStatus) error {
	maplock.Lock()
	for _, ch := range s.statusStreams {
		ch <- status
	}
	maplock.Unlock()

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
