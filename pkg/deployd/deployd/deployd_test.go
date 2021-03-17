package deployd_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"sync"
	"testing"
	"time"

	"github.com/nais/deploy/pkg/deployd/deployd"
	"github.com/nais/deploy/pkg/deployd/kubeclient"
	"github.com/nais/deploy/pkg/deployd/operation"
	"github.com/nais/deploy/pkg/pb"
	nais_io_v1alpha1 "github.com/nais/liberator/pkg/apis/nais.io/v1alpha1"
	"github.com/nais/liberator/pkg/crd"
	"github.com/nais/liberator/pkg/events"
	"github.com/nais/liberator/pkg/keygen"
	"github.com/nais/liberator/pkg/scheme"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

type processCallback func(ctx context.Context, rig *testRig, test testSpec) error

type testSpec struct {
	fixture           string               // path to testdata to deploy to cluster
	timeout           time.Duration        // time to allow for deploy to reach end state
	endStatus         *pb.DeploymentStatus // which end state we expect
	deployedResources []runtime.Object     // list of Kubernetes resources expected to be applied to the cluster - only checks name and namespace
	processing        processCallback      // processing that happens in a coroutine together with deployd.Run(). Requires all resources in `deployedResources` to exist.
}

var tests = []testSpec{
	// Simple configmap with no watcher
	{
		fixture: "testdata/configmap.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State: pb.DeploymentState_success,
		},
		deployedResources: []runtime.Object{
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "default",
				},
			},
		},
	},

	// Check that deploy times out
	{
		fixture: "testdata/application-timeout.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_failure,
			Message: "timeout while waiting for deployment to succeed (total of 1 errors)",
		},
		deployedResources: []runtime.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-timeout",
					Namespace: "default",
				},
			},
		},
	},

	// Happy path for applications
	{
		fixture: "testdata/application-rolloutcomplete.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_success,
			Message: "Deployment completed successfully.",
		},
		deployedResources: []runtime.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication",
					Namespace: "default",
				},
			},
		},
		processing: func(ctx context.Context, rig *testRig, test testSpec) error {
			return rig.client.Create(ctx, naiseratorEvent(test.fixture, events.RolloutComplete, "completed", "myapplication"))
		},
	},

	// Application failed synchronization in Naiserator
	{
		fixture: "testdata/application-failedsynchronization.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_failure,
			Message: "Application/myapplication-failedsynchronization (FailedSynchronization): oops (total of 1 errors)",
		},
		deployedResources: []runtime.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-failedsynchronization",
					Namespace: "default",
				},
			},
		},
		processing: func(ctx context.Context, rig *testRig, test testSpec) error {
			return rig.client.Create(ctx, naiseratorEvent(test.fixture, events.FailedSynchronization, "oops", "myapplication-failedsynchronization"))
		},
	},

	// Application failed prep in Naiserator
	{
		fixture: "testdata/application-failedprepare.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_failure,
			Message: "Application/myapplication-failedprepare (FailedPrepare): oops (total of 1 errors)",
		},
		deployedResources: []runtime.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-failedprepare",
					Namespace: "default",
				},
			},
		},
		processing: func(ctx context.Context, rig *testRig, test testSpec) error {
			return rig.client.Create(ctx, naiseratorEvent(test.fixture, events.FailedPrepare, "oops", "myapplication-failedprepare"))
		},
	},
}

type testRig struct {
	kubernetes *envtest.Environment
	client     client.Client
	structured kubernetes.Interface
	dynamic    dynamic.Interface
	statusChan chan *pb.DeploymentStatus
	kubeclient kubeclient.Interface
	scheme     *runtime.Scheme
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

	rig.scheme, err = scheme.All()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes schemes: %s", err)
	}

	rig.client, err = client.New(cfg, client.Options{
		Scheme: rig.scheme,
	})
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes client: %w", err)
	}

	rig.kubeclient, err = kubeclient.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("initialize custom client: %w", err)
	}

	rig.statusChan = make(chan *pb.DeploymentStatus, 16)

	return rig, nil
}

func resources(path string) (*pb.Kubernetes, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return pb.KubernetesFromJSONResources(data)
}

func compareStatus(expected, actual *pb.DeploymentStatus) error {
	if expected.GetState() != actual.GetState() {
		return fmt.Errorf("expected state '%s' but got '%s'", expected.GetState(), actual.GetState())
	}
	if len(expected.GetMessage()) > 0 && expected.GetMessage() != actual.GetMessage() {
		return fmt.Errorf("expected status message '%s' but got '%s'", expected.GetMessage(), actual.GetMessage())
	}
	return nil
}

func waitFinish(statusChan <-chan *pb.DeploymentStatus, expectedStatus *pb.DeploymentStatus) error {
	for status := range statusChan {
		state := status.GetState()
		log.Infof("Deployment status '%s': %s", state, status.GetMessage())
		if state.Finished() {
			log.Infof("Deploy reached finished state")
			return compareStatus(expectedStatus, status)
		}
	}
	return fmt.Errorf("channel closed but no end state")
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

func resourceExists(ctx context.Context, rig *testRig, resource runtime.Object) error {
	key, err := client.ObjectKeyFromObject(resource)
	if err != nil {
		panic(fmt.Sprintf("test data error: %s", err))
	}
	obj := resource.DeepCopyObject()
	return rig.client.Get(ctx, key, obj)
}

func waitForResources(ctx context.Context, rig *testRig, test testSpec) error {
	var err error

	for ctx.Err() == nil {
		// Check that resources was deployed to the cluster
		for i, expectedDeployed := range test.deployedResources {
			err = resourceExists(ctx, rig, expectedDeployed)
			if err != nil {
				err = fmt.Errorf("resource %d: %s", i, err)
				break
			}
		}
		if err == nil {
			return nil
		}
	}
	return ctx.Err()
}

func subTest(t *testing.T, rig *testRig, test testSpec) {
	ctx, cancel := context.WithTimeout(context.Background(), test.timeout)
	defer cancel()

	kubes, err := resources(test.fixture)
	if err != nil {
		panic(fmt.Sprintf("test data fixture error in '%s': %s", test.fixture, err))
	}

	op := &operation.Operation{
		Context: ctx,
		Logger:  log.WithField("fixture", test.fixture),
		Request: &pb.DeploymentRequest{
			ID:         test.fixture,
			Kubernetes: kubes,
		},
		StatusChan: rig.statusChan,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)

	go func() {
		defer wg.Done()
		err := waitForResources(ctx, rig, test)
		if err != nil {
			t.Errorf("Wait for resources: %s", err)
			t.Fail()
			return
		}
		log.Infof("Resources rolled out, running coroutine processing")
		if test.processing != nil {
			err = test.processing(ctx, rig, test)
			assert.NoError(t, err)
		}
		log.Infof("Coroutine processing finished")
	}()

	// Start deployment
	deployd.Run(op, rig.kubeclient)
	err = waitFinish(rig.statusChan, test.endStatus)
	assert.NoError(t, err)

	wg.Wait()
}

func naiseratorEvent(id, reason, message, app string) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-" + keygen.RandStringBytes(10),
			Namespace: "default",
			Annotations: map[string]string{
				nais_io_v1alpha1.DeploymentCorrelationIDAnnotation: id,
			},
		},
		ReportingController: "naiserator",
		Reason:              reason,
		Message:             message,
		InvolvedObject: v1.ObjectReference{
			Kind: "Application",
			Name: app,
		},
		LastTimestamp: metav1.NewTime(time.Now()),
	}
}
