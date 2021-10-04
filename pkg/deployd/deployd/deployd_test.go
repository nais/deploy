package deployd_test

import (
	"context"
	"fmt"
	nais_io_v1 "github.com/nais/liberator/pkg/apis/nais.io/v1"
	"io/ioutil"
	rbac_v1 "k8s.io/api/rbac/v1"
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
	deployedResources []client.Object      // list of Kubernetes resources expected to be applied to the cluster - only checks name and namespace
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
		deployedResources: []client.Object{
			&v1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "foo",
					Namespace: "aura",
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
		deployedResources: []client.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-timeout",
					Namespace: "aura",
				},
			},
		},
	},

	// Deployments to other namespaces are unauthorized
	{
		fixture: "testdata/application-unauthorized.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_failure,
			Message: "nais.io/v1alpha1, Kind=Application, Namespace=not-aura, Name=myapplication-unauthorized: get existing resource: applications.nais.io \"myapplication-unauthorized\" is forbidden: User \"system:serviceaccount:aura:serviceuser-aura\" cannot get resource \"applications\" in API group \"nais.io\" in the namespace \"not-aura\" (total of 1 errors)",
		},
		deployedResources: nil,
	},

	// Happy path for applications
	{
		fixture: "testdata/application-rolloutcomplete.json",
		timeout: 2 * time.Second,
		endStatus: &pb.DeploymentStatus{
			State:   pb.DeploymentState_success,
			Message: "Deployment completed successfully.",
		},
		deployedResources: []client.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication",
					Namespace: "aura",
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
		deployedResources: []client.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-failedsynchronization",
					Namespace: "aura",
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
		deployedResources: []client.Object{
			&nais_io_v1alpha1.Application{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myapplication-failedprepare",
					Namespace: "aura",
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
	var err error

	rig := &testRig{}

	rig.scheme, err = scheme.All()
	if err != nil {
		return nil, fmt.Errorf("initialize Kubernetes schemes: %s", err)
	}

	crdPath := crd.YamlDirectory()
	rig.kubernetes = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			Scheme: rig.scheme,
			Paths:  []string{crdPath},
		},
	}

	cfg, err := rig.kubernetes.Start()
	if err != nil {
		return nil, fmt.Errorf("setup Kubernetes test environment: %w", err)
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

func createRBAC(ctx context.Context, team string, rig *testRig) error {
	rbac := &rbac_v1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "allow-rest-mapper",
			Namespace: team,
		},
		Subjects: []rbac_v1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "serviceuser-" + team,
				Namespace: team,
			},
		},
		RoleRef: rbac_v1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}

	return rig.client.Create(ctx, rbac)
}

func createNamespace(ctx context.Context, namespace string, rig *testRig) error {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	return rig.client.Create(ctx, ns)
}

func createServiceAccount(ctx context.Context, team string, rig *testRig) error {
	name := "serviceuser-" + team

	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: team,
			Annotations: map[string]string{
				"kubernetes.io/service-account.name": name,
			},
		},
		StringData: map[string]string{
			"ca.crt":    string(rig.kubernetes.Config.CAData),
			"namespace": team,
			"token":     "",
		},
		Type: v1.SecretTypeServiceAccountToken,
	}

	err := rig.client.Create(ctx, secret)
	if err != nil {
		return err
	}

	svcacc := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: team,
		},
		Secrets: []v1.ObjectReference{
			{
				Kind:            secret.Kind,
				Namespace:       secret.Namespace,
				Name:            secret.Name,
				UID:             secret.GetUID(),
				APIVersion:      secret.APIVersion,
				ResourceVersion: secret.GetResourceVersion(),
			},
		},
	}

	err = rig.client.Create(ctx, svcacc)
	if err != nil {
		return err
	}

	return nil
}

// This test sets up a complete in-memory Kubernetes rig, and tests the deploy and watch strategies against it.
// These tests ensure that resources are actually created or updated in the cluster.
func TestDeployRun(t *testing.T) {
	rig, err := newTestRig()
	if err != nil {
		t.Errorf("unable to run strategy integration tests: %s", err)
		t.FailNow()
	}

	t.Logf("Test rig initialized")

	defer rig.kubernetes.Stop()

	log.SetLevel(log.TraceLevel)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	team := "aura"

	t.Logf("Creating namespace")
	err = createNamespace(ctx, team, rig)
	if err != nil {
		t.Errorf("unable to create namespace: %s", err)
		t.FailNow()
	}

	t.Logf("Creating role binding")
	err = createRBAC(ctx, team, rig)
	if err != nil {
		t.Errorf("unable to create RBAC: %s", err)
		t.FailNow()
	}

	// set up teams for impersonation
	t.Logf("Creating service account")
	err = createServiceAccount(ctx, team, rig)
	if err != nil {
		t.Errorf("unable to create service account: %s", err)
		t.FailNow()
	}
	time.Sleep(1 * time.Second)

	for _, test := range tests {
		subTest(t, rig, test, team)
	}

	if err := recover(); err != nil {
		t.Error(err)
		t.Fail()
	}
}

func resourceExists(ctx context.Context, rig *testRig, resource client.Object) error {
	key := client.ObjectKeyFromObject(resource)
	obj := resource.DeepCopyObject()
	return rig.client.Get(ctx, key, obj.(client.Object))
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

func subTest(t *testing.T, rig *testRig, test testSpec, team string) {
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
			Team:       team,
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
	teamClient, err := rig.kubeclient.Impersonate(op.Request.GetTeam())
	assert.NoError(t, err)
	if err != nil {
		return
	}
	deployd.Run(op, teamClient)

	err = waitFinish(rig.statusChan, test.endStatus)
	assert.NoError(t, err)

	wg.Wait()
}

func naiseratorEvent(id, reason, message, app string) *v1.Event {
	return &v1.Event{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "event-" + keygen.RandStringBytes(10),
			Namespace: "aura",
			Annotations: map[string]string{
				nais_io_v1.DeploymentCorrelationIDAnnotation: id,
			},
		},
		ReportingController: "naiserator",
		Reason:              reason,
		Message:             message,
		InvolvedObject: v1.ObjectReference{
			Kind:      "Application",
			Namespace: "aura",
			Name:      app,
		},
		LastTimestamp: metav1.NewTime(time.Now()),
	}
}
