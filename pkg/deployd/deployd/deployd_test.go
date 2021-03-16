package deployd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/nais/deploy/pkg/deployd/deployd"
	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	"github.com/nais/liberator/pkg/crd"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type testSpec struct {
	fixture           string             // path to testdata to deploy to cluster
	timeout           time.Duration      // time to allow for deploy to reach end state
	endState          pb.DeploymentState // which end state we expect
	deployedResources []runtime.Object   // list of Kubernetes resources expected to be applied to the cluster - only checks name and namespace
}

var tests = []testSpec{
	{
		fixture:  "testdata/configmap.json",
		timeout:  1 * time.Second,
		endState: pb.DeploymentState_success,
		deployedResources: []runtime.Object{
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
			},
		},
	},
}

type testRig struct {
	kubernetes   *envtest.Environment
	client       client.Client
	structured   kubernetes.Interface
	dynamic      dynamic.Interface
	synchronizer reconcile.Reconciler
	statusChan   chan *pb.DeploymentStatus
	teamClient   kubeclient.TeamClient
	scheme       *runtime.Scheme
}

func newTestRig() (*testRig, error) {
	rig := &testRig{}
	crdPath := crd.YamlDirectory()
	rig.kubernetes = &envtest.Environment{
		CRDDirectoryPaths: []string{crdPath},
	}

	cfg, err := rig.kubernetes.Start()
	if err != nil {
		return nil, fmt.Errorf("setup Kubernetes test environment: %w", err)
	}

	rig.scheme = clientgoscheme.Scheme

	rig.client, err = client.New(cfg, client.Options{
		Scheme: rig.scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client: %w", err)
	}

	rig.structured, err = kubernetes.NewForConfig(rig.kubernetes.Config)
	if err != nil {
		return nil, fmt.Errorf("initialize structured client: %w", err)
	}

	rig.dynamic, err = dynamic.NewForConfig(rig.kubernetes.Config)
	if err != nil {
		return nil, fmt.Errorf("initialize dynamic client: %w", err)
	}

	rig.statusChan = make(chan *pb.DeploymentStatus, 16)
	rig.teamClient = kubeclient.NewTeamClient(rig.structured, rig.dynamic)

	return rig, nil
}

func resources(path string) (*pb.Kubernetes, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return pb.KubernetesFromJSONResources(data)
}

func waitFinish(ctx context.Context, statusChan <-chan *pb.DeploymentStatus, expectedState pb.DeploymentState) error {
	for {
		select {
		case status := <-statusChan:
			state := status.GetState()
			log.Infof("Deployment status '%s': %s", state, status.GetMessage())
			if state.Finished() {
				if state == expectedState {
					log.Infof("Deploy reached finished state")
					return nil
				}
				return fmt.Errorf("Expected deployment to end with '%s' but got '%s'", expectedState, state)
			}

		case <-ctx.Done():
			return fmt.Errorf("Timed out waiting for finished state")
		}
	}
}

// This test sets up a complete in-memory Kubernetes rig, and tests the deploy and watch strategies against it.
// These tests ensure that resources are actually created or updated in the cluster.
func TestDeployRun(t *testing.T) {
	rig, err := newTestRig()
	if err != nil {
		t.Errorf("unable to run strategy integration tests: %s", err)
		t.FailNow()
	}

	defer rig.kubernetes.Stop()

	log.SetLevel(log.TraceLevel)

	for _, test := range tests {
		subTest(t, rig, test)
	}

	if err := recover(); err != nil {
		t.Error(err)
		t.Fail()
	}
}

func subTest(t *testing.T, rig *testRig, test testSpec) {
	// Allow no more than 15 seconds for these tests to run
	ctx, cancel := context.WithTimeout(context.Background(), test.timeout)
	defer cancel()

	kubes, err := resources(test.fixture)
	if err != nil {
		panic(fmt.Sprintf("test data fixture error in '%s': %s", test.fixture, err))
	}

	op := &operation.Operation{
		Context: ctx,
		Logger:  log.NewEntry(log.StandardLogger()),
		Request: &pb.DeploymentRequest{
			Kubernetes: kubes,
		},
		StatusChan: rig.statusChan,
	}

	// Start deployment
	deployd.Run(op, rig.teamClient)
	err = waitFinish(ctx, rig.statusChan, test.endState)
	assert.NoError(t, err)

	// Check that resources was deployed to the cluster
	for i, expectedDeployed := range test.deployedResources {
		key, err := client.ObjectKeyFromObject(expectedDeployed)
		if err != nil {
			panic(fmt.Sprintf("test data error in resource %d: %s", i, err))
		}
		obj := expectedDeployed.DeepCopyObject()
		err = rig.client.Get(ctx, key, obj)
		assert.NoError(t, err)
	}
}
