package deployer_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/navikt/deployment/pkg/deployer"
	"github.com/navikt/deployment/pkg/pb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeMockDeployRequest(cfg deployer.Config) *pb.DeploymentRequest {
	tm := time.Now()
	deadline := time.Now().Add(1 * time.Minute)
	request := deployer.MakeDeploymentRequest(cfg, deadline, &pb.Kubernetes{})
	request.Time = pb.TimeAsTimestamp(tm)
	return request
}

func TestSimpleSuccessfulDeploy(t *testing.T) {
	cfg := validConfig()
	request := makeMockDeployRequest(*cfg)
	ctx := context.Background()

	client := &pb.MockDeployClient{}
	client.On("Deploy", ctx, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "happy",
	}, nil).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.NoError(t, err)
	assert.Equal(t, deployer.ExitSuccess, deployer.ErrorExitCode(err))
}

func TestSuccessfulDeploy(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()

	client := &pb.MockDeployClient{}
	client.On("Deploy", ctx, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
	}, nil).Once()

	statusClient := &pb.MockDeploy_StatusClient{}
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "happy",
	}, nil).Once()

	client.On("Status", ctx, request).Return(statusClient, nil).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.NoError(t, err)
	assert.Equal(t, deployer.ExitSuccess, deployer.ErrorExitCode(err))
}

func TestDeployError(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()

	client := &pb.MockDeployClient{}
	client.On("Deploy", ctx, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
		Message: "queued",
	}, nil).Once()

	statusClient := &pb.MockDeploy_StatusClient{}
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_error,
		Message: "oops, we errored out",
	}, nil).Once()

	client.On("Status", ctx, request).Return(statusClient, nil).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.Error(t, err)
	assert.Equal(t, deployer.ExitDeploymentError, deployer.ErrorExitCode(err))
}

func TestDeployPolling(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()

	client := &pb.MockDeployClient{}
	client.On("Deploy", ctx, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
		Message: "queued",
	}, nil).Once()

	statusClient := &pb.MockDeploy_StatusClient{}
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_in_progress,
		Message: "working...",
	}, nil).Times(5)
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "finally over",
	}, nil).Once()

	client.On("Status", ctx, request).Return(statusClient, nil).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.NoError(t, err)
	assert.Equal(t, deployer.ExitSuccess, deployer.ErrorExitCode(err))
}

func TestDeployWithStatusRetry(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	cfg.RetryInterval = time.Millisecond * 50
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()

	client := &pb.MockDeployClient{}

	client.On("Deploy", ctx, request).Return(nil, status.Errorf(codes.Unavailable, "we are suffering from instability")).Times(3)
	client.On("Deploy", ctx, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
		Message: "queued",
	}, nil).Once()

	statusClient := &pb.MockDeploy_StatusClient{}

	// set up status stream
	client.On("Status", ctx, request).Return(nil, status.Errorf(codes.Unavailable, "oops, more errors")).Times(2)
	client.On("Status", ctx, request).Return(statusClient, nil).Once()

	// poll a few times
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_in_progress,
		Message: "working...",
	}, nil).Times(2)

	// server goes down
	statusClient.On("Recv").Return(nil, status.Errorf(codes.Unavailable, "not so fast, young man")).Once()

	// re-establish status stream
	client.On("Status", ctx, request).Return(nil, status.Errorf(codes.Unavailable, "still down")).Times(3)
	client.On("Status", ctx, request).Return(statusClient, nil).Once()

	// come back to discover a successful deployment
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "finally over",
	}, nil).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.NoError(t, err)
	assert.Equal(t, deployer.ExitSuccess, deployer.ErrorExitCode(err))
}

func TestImmediateTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"

	// time out the request
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client := &pb.MockDeployClient{}

	client.On("Deploy", ctx, request).Return(nil, status.Errorf(codes.DeadlineExceeded, "too slow, mofo")).Once()

	d := deployer.Deployer{Client: client}
	err := d.Deploy(ctx, request)

	assert.Error(t, err)
	assert.Equal(t, deployer.ExitTimeout, deployer.ErrorExitCode(err))
}

func TestPrepareJSON(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/alert.json"}

	d := deployer.Deployer{}

	request, err := d.Prepare(context.Background(), *cfg)

	assert.NoError(t, err)

	assert.Equal(t, "aura", request.Team, "auto-detection of team works")
	assert.Equal(t, "master", request.GitRefSha, "defaulting works")
	assert.Equal(t, "dev-fss", request.GithubEnvironment, "auto-detection of environment works")
	assert.Equal(t, cfg.Cluster, request.Cluster, "cluster is set")
}

func TestPrepareYAML(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}

	d := deployer.Deployer{}

	request, err := d.Prepare(context.Background(), *cfg)

	assert.NoError(t, err)

	assert.Equal(t, "aura", request.Team, "auto-detection of team works")
	assert.Equal(t, "master", request.GitRefSha, "defaulting works")
	assert.Equal(t, "dev-fss:nais", request.GithubEnvironment, "auto-detection of environment works")
	assert.Equal(t, cfg.Cluster, request.Cluster, "cluster is set")
}

func TestValidationFailures(t *testing.T) {
	valid := validConfig()
	d := deployer.Deployer{}

	for _, testCase := range []struct {
		errorMsg  string
		transform func(cfg deployer.Config) deployer.Config
	}{
		{deployer.ClusterRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Cluster = ""; return cfg }},
		{deployer.APIKeyRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = ""; return cfg }},
		{deployer.ResourceRequiredMsg, func(cfg deployer.Config) deployer.Config { cfg.Resource = nil; return cfg }},
		{deployer.MalformedAPIKeyMsg, func(cfg deployer.Config) deployer.Config { cfg.APIKey = "malformed"; return cfg }},
	} {
		cfg := testCase.transform(*valid)
		request, err := d.Prepare(context.Background(), cfg)
		assert.Error(t, err)
		assert.Nil(t, request)
		assert.Equal(t, deployer.ExitInvocationFailure, deployer.ErrorExitCode(err))
		assert.Contains(t, err.Error(), testCase.errorMsg)
	}
}

func TestTemplateVariableFromCommandLine(t *testing.T) {
	type Vars struct {
		One   string
		Two   string
		Three string
		Four  string
	}
	cfg := validConfig()
	cfg.Team = "foo"
	cfg.Resource = []string{"testdata/templated.yaml"}
	cfg.VariablesFile = "testdata/vars.yaml"
	cfg.Variables = []string{
		"one=ONE",
		"two=TWO",
	}

	d := deployer.Deployer{}
	request, err := d.Prepare(context.Background(), *cfg)
	assert.NoError(t, err)

	resources, err := request.Kubernetes.JSONResources()
	assert.NoError(t, err)

	vars := &Vars{}
	err = json.Unmarshal(resources[0], vars)
	assert.NoError(t, err)

	assert.Equal(t, "ONE", vars.One)
	assert.Equal(t, "TWO", vars.Two)
	assert.Equal(t, "THREE", vars.Three)
	assert.Equal(t, "FOUR", vars.Four)
}

func TestExitCodeZero(t *testing.T) {
	assert.Equal(t, deployer.ExitCode(0), deployer.ExitSuccess)
}

func validConfig() *deployer.Config {
	cfg := deployer.NewConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}
	cfg.Cluster = "dev-fss"
	cfg.Repository = "myrepo"
	cfg.APIKey = "1234567812345678"
	return cfg
}
