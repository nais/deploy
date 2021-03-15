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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type testRig struct {
	kubernetes   *envtest.Environment
	client       client.Client
	structured   kubernetes.Interface
	dynamic      dynamic.Interface
	synchronizer reconcile.Reconciler
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

	return rig, nil
}

func resources(path string) (*pb.Kubernetes, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return pb.KubernetesFromJSONResources(data)
}

func waitFinish(ctx context.Context, statusChan <-chan *pb.DeploymentStatus) error {
	for {
		select {
		case status := <-statusChan:
			log.Infof("Deployment status '%s': %s", status.GetState(), status.GetMessage())
			if status.GetState().Finished() {
				log.Infof("Deploy reached finished state, exiting")
				return nil
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

	// Allow no more than 15 seconds for these tests to run
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*615)
	defer cancel()

	kubes, err := resources("testdata/configmap.json")
	if err != nil {
		panic(fmt.Sprintf("test data fixture error: %s", err))
	}

	log.SetLevel(log.TraceLevel)
	statusChan := make(chan *pb.DeploymentStatus, 16)
	teamClient := kubeclient.NewTeamClient(rig.structured, rig.dynamic)
	defer close(statusChan)

	op := &operation.Operation{
		Context: ctx,
		Logger:  log.NewEntry(log.StandardLogger()),
		Request: &pb.DeploymentRequest{
			Kubernetes: kubes,
		},
		StatusChan: statusChan,
	}

	// Start deployment
	deployd.Run(op, teamClient)
	err = waitFinish(ctx, statusChan)
	assert.NoError(t, err)

	// Check that the resource was deployed to the cluster
	obj := &v1.ConfigMap{}
	err = rig.client.Get(ctx, client.ObjectKey{Name: "foo", Namespace: "default"}, obj)
	assert.NoError(t, err)
	assert.Equal(t, "bar", obj.Data["foo"])
}
