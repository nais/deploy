package deployclient_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nais/deploy/pkg/deployclient"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/deploy/pkg/telemetry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeMockDeployRequest(cfg deployclient.Config) *pb.DeploymentRequest {
	tm := time.Now()
	deadline := time.Now().Add(1 * time.Minute)
	request := deployclient.MakeDeploymentRequest(cfg, deadline, &pb.Kubernetes{})
	request.Time = pb.TimeAsTimestamp(tm)
	return request
}

func TestSimpleSuccessfulDeploy(t *testing.T) {
	cfg := validConfig()
	request := makeMockDeployRequest(*cfg)
	ctx := context.Background()
	_, _ = telemetry.New(ctx, "test", "")

	client := &pb.MockDeployClient{}
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "happy",
	}, nil).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.NoError(t, err)
	assert.Equal(t, deployclient.ExitSuccess, deployclient.ErrorExitCode(err))
}

func TestSuccessfulDeploy(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()
	_, _ = telemetry.New(ctx, "test", "")

	client := &pb.MockDeployClient{}
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
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

	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.NoError(t, err)
	assert.Equal(t, deployclient.ExitSuccess, deployclient.ErrorExitCode(err))
}

func TestDeployError(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()
	_, _ = telemetry.New(ctx, "test", "")

	client := &pb.MockDeployClient{}
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
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

	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.Error(t, err)
	assert.Equal(t, deployclient.ExitDeploymentError, deployclient.ErrorExitCode(err))
}

func TestDeployPolling(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()
	_, _ = telemetry.New(ctx, "test", "")

	client := &pb.MockDeployClient{}
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
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

	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.NoError(t, err)
	assert.Equal(t, deployclient.ExitSuccess, deployclient.ErrorExitCode(err))
}

func TestDeployWithStatusRetry(t *testing.T) {
	cfg := validConfig()
	cfg.Retry = true
	cfg.Wait = true
	cfg.RetryInterval = time.Millisecond * 50
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"
	ctx := context.Background()
	_, _ = telemetry.New(ctx, "test", "")

	client := &pb.MockDeployClient{}

	client.On("Deploy", mock.Anything, request).Return(nil, status.Errorf(codes.Unavailable, "we are suffering from instability")).Times(3)
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
		Message: "queued",
	}, nil).Once()

	statusClient := &pb.MockDeploy_StatusClient{}

	// set up status stream
	client.On("Status", mock.Anything, request).Return(nil, status.Errorf(codes.Unavailable, "oops, more errors")).Times(2)
	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

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
	client.On("Status", mock.Anything, request).Return(nil, status.Errorf(codes.Unavailable, "still down")).Times(3)
	client.On("Status", mock.Anything, request).Return(nil, status.Errorf(codes.Internal, "still down, internal error")).Times(3)
	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

	// more internal errors in stream
	statusClient.On("Recv").Return(nil, status.Errorf(codes.Internal, "internal error again")).Once()

	// re-establish status stream
	client.On("Status", mock.Anything, request).Return(statusClient, nil).Once()

	// come back to discover deployment is gone
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_inactive,
		Message: "up again but lost",
	}, nil).Once()

	// re-send deployment request
	client.On("Deploy", mock.Anything, request).Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_queued,
		Message: "queued",
	}, nil).Once()

	// come back to discover a successful deployment
	statusClient.On("Recv").Return(&pb.DeploymentStatus{
		Request: request,
		Time:    pb.TimeAsTimestamp(time.Now()),
		State:   pb.DeploymentState_success,
		Message: "finally over",
	}, nil).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.NoError(t, err)
	assert.Equal(t, deployclient.ExitSuccess, deployclient.ErrorExitCode(err))
}

func TestImmediateTimeout(t *testing.T) {
	cfg := validConfig()
	cfg.Wait = true
	request := makeMockDeployRequest(*cfg)
	request.ID = "1"

	// time out the request
	ctx, cancel := context.WithCancel(context.Background())
	_, _ = telemetry.New(ctx, "test", "")
	cancel()

	client := &pb.MockDeployClient{}

	client.On("Deploy", mock.Anything, request).Return(nil, status.Errorf(codes.DeadlineExceeded, "too slow, mofo")).Once()

	d := deployclient.Deployer{Client: client}
	err := d.Deploy(ctx, cfg, request)

	assert.Error(t, err)
	assert.Equal(t, deployclient.ExitTimeout, deployclient.ErrorExitCode(err))
}

func TestPrepareJSON(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/alert.json"}

	request, err := deployclient.Prepare(context.Background(), cfg)

	assert.NoError(t, err)

	assert.Equal(t, "aura", request.Team, "auto-detection of team works")
	assert.Equal(t, "dev-fss", request.GithubEnvironment, "auto-detection of environment works")
	assert.Equal(t, cfg.Cluster, request.Cluster, "cluster is set")
}

func TestPrepareYAML(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}

	request, err := deployclient.Prepare(context.Background(), cfg)

	assert.NoError(t, err)

	assert.Equal(t, "aura", request.Team, "auto-detection of team works")
	assert.Equal(t, "dev-fss:nais", request.GithubEnvironment, "auto-detection of environment works")
	assert.Equal(t, cfg.Cluster, request.Cluster, "cluster is set")
}

func TestAnnotationInjection(t *testing.T) {
	cfg := validConfig()
	cfg.Resource = []string{"testdata/alert.json"}

	request, err := deployclient.Prepare(context.Background(), cfg)

	assert.NoError(t, err)

	assert.Equal(t, "aura", request.Team, "auto-detection of team works")
	assert.Equal(t, "dev-fss", request.GithubEnvironment, "auto-detection of environment works")
	assert.Equal(t, cfg.Cluster, request.Cluster, "cluster is set")
}

func TestValidationFailures(t *testing.T) {
	valid := validConfig()

	for _, testCase := range []struct {
		errorMsg  string
		transform func(cfg deployclient.Config) deployclient.Config
	}{
		{deployclient.ErrClusterRequired.Error(), func(cfg deployclient.Config) deployclient.Config { cfg.Cluster = ""; return cfg }},
		{deployclient.ErrAuthRequired.Error(), func(cfg deployclient.Config) deployclient.Config { cfg.APIKey = ""; return cfg }},
		{deployclient.ErrResourceRequired.Error(), func(cfg deployclient.Config) deployclient.Config { cfg.Resource = nil; return cfg }},
		{deployclient.ErrMalformedAPIKey.Error(), func(cfg deployclient.Config) deployclient.Config { cfg.APIKey = "malformed"; return cfg }},
	} {
		cfg := testCase.transform(*valid)
		request, err := deployclient.Prepare(context.Background(), &cfg)
		assert.Error(t, err)
		assert.Nil(t, request)
		assert.Equal(t, deployclient.ExitInvocationFailure, deployclient.ErrorExitCode(err))
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

	request, err := deployclient.Prepare(context.Background(), cfg)
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
	assert.Equal(t, deployclient.ExitCode(0), deployclient.ExitSuccess)
}

func validConfig() *deployclient.Config {
	cfg := deployclient.NewConfig()
	cfg.Resource = []string{"testdata/nais.yaml"}
	cfg.Cluster = "dev-fss"
	cfg.Repository = "myrepo"
	cfg.APIKey = "1234567812345678"
	return cfg
}
