// package deployServer provides message streams between hookd and deployd

package dispatchserver

import (
	"context"
	"fmt"
	"time"

	"github.com/nais/deploy/pkg/hookd/database"
	database_mapper "github.com/nais/deploy/pkg/hookd/database/mapper"
	"github.com/nais/deploy/pkg/hookd/github"
	"github.com/nais/deploy/pkg/hookd/metrics"
	"github.com/nais/deploy/pkg/pb"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	requestTimeout          = time.Second * 5
	errNoRepository         = fmt.Errorf("no repository specified")
	errNoGithubDeploymentID = fmt.Errorf("GitHub deployment ID not recorded in database")
)

func (s *dispatchServer) githubLoop() {
	// State cache. Only report one of each kind of state for each deployment.
	// This is neccessary to avoid rate limiting.
	statuses := make(map[string]pb.DeploymentState)

	for {
		select {
		case request := <-s.requests:
			logger := log.WithFields(request.LogFields())
			err := s.createGithubDeployment(request)
			switch err {
			case errNoRepository:
				logger.Debugf("Not syncing deployment to GitHub: %s", err)
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
				logger.Debugf("Synchronized deployment to GitHub")
			default:
				logger.Errorf("Unable to sync deployment to GitHub: %s", err)
			}

		case st := <-s.statuses:
			logger := log.WithFields(st.LogFields())

			reqid := st.GetRequest().GetID()
			lastState, hasLastState := statuses[reqid]
			if hasLastState && lastState == st.GetState() {
				logger.Tracef("Not syncing deployment status to GitHub: last state for this deployment is identical")
				break
			}

			err := s.createGithubDeploymentStatus(st)
			switch err {
			case errNoRepository, errNoGithubDeploymentID:
				logger.Debugf("Not syncing deployment status to GitHub: %s", err)
			case nil:
				logger.Debugf("Synchronized deployment status to GitHub")
				statuses[reqid] = st.GetState()
			default:
				logger.Errorf("Unable to sync deployment status to GitHub: %s", err)
			}

			// Clean up state cache to avoid memory leaks
			if st.GetState().Finished() {
				delete(statuses, reqid)
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
		return errNoGithubDeploymentID
	}

	deploymentID := int64(*deploy.GitHubID)
	_, err = s.githubClient.CreateDeploymentStatus(ctx, status, deploymentID)
	if err != nil {
		return fmt.Errorf("create GitHub deployment status: %s", err)
	}

	return nil
}

func (s *dispatchServer) SendDeploymentRequest(ctx context.Context, request *pb.DeploymentRequest) error {
  s.dispatchStreamsLock.RLock()
  stream, online := s.dispatchStreams[request.Cluster]
  s.dispatchStreamsLock.RUnlock()
  if !online{
    return status.Errorf(codes.Unavailable, "cluster '%s' is offline", request.Cluster)
  }

  err := stream.Send(request)
	if err != nil {
		return err
	}

	s.requests <- request
	log.WithFields(request.LogFields()).Debugf("Deployment request sent to deployd")

	return nil
}

func (s *dispatchServer) HandleDeploymentStatus(ctx context.Context, st *pb.DeploymentStatus) error {
  s.statusStreamsLock.RLock()
	for _, ch := range s.statusStreams {
		ch <- st
	}
  s.statusStreamsLock.RUnlock()

	dbStatus := database_mapper.DeploymentStatus(st)
	err := s.db.WriteDeploymentStatus(ctx, dbStatus)
	if err != nil {
		if database.IsErrForeignKeyViolation(err) {
			return status.Errorf(codes.FailedPrecondition, err.Error())
		}
		return status.Errorf(codes.Unavailable, "write deployment status to database: %s", err)
	}

	metrics.UpdateQueue(st)

	logger := log.WithFields(st.LogFields())
	logger.Debugf("Saved deployment status in database")

	if st.GetState().Finished() {
		logger.Infof("Deployment finished")
	}

	s.statuses <- st

	return nil
}
