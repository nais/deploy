package deployd

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/navikt/deployment/pkg/pb"
	"github.com/navikt/deployment/pkg/deployd/config"
	"github.com/navikt/deployment/pkg/deployd/kubeclient"
	"github.com/navikt/deployment/pkg/deployd/metrics"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	ErrNotMyCluster     = fmt.Errorf("your message belongs in another cluster")
	ErrDeadlineExceeded = fmt.Errorf("deadline exceeded")

	deploymentTimeout = time.Minute * 30
)

const (
	DefaultTeamclientNamespace = "default"
)

func matchesCluster(req pb.DeploymentRequest, cluster string) error {
	if req.GetCluster() != cluster {
		return ErrNotMyCluster
	}
	return nil
}

func meetsDeadline(req pb.DeploymentRequest) error {
	deadline := time.Unix(req.GetDeadline(), 0)
	late := time.Since(deadline)
	if late > 0 {
		return ErrDeadlineExceeded
	}
	return nil
}

func jsonToResources(json []json.RawMessage) ([]unstructured.Unstructured, error) {
	resources := make([]unstructured.Unstructured, len(json))
	for i := range resources {
		err := resources[i].UnmarshalJSON(json[i])
		if err != nil {
			return nil, fmt.Errorf("resource %d: decoding payload: %s", i+1, err)
		}
	}
	return resources, nil
}

// Annotate a resource with the deployment correlation ID.
func addCorrelationID(resource *unstructured.Unstructured, correlationID string) {
	anno := resource.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	anno[kubeclient.CorrelationIDAnnotation] = correlationID
	resource.SetAnnotations(anno)
}

// Prepare decodes a string of bytes into a deployment request,
// and decides whether or not to allow a deployment.
//
// If everything is okay, returns a deployment request. Otherwise, an error.
func Prepare(req *pb.DeploymentRequest, cluster string) error {
	// Check if we are the authoritative handler for this message
	if err := matchesCluster(*req, cluster); err != nil {
		return err
	}

	// Messages that are too old are discarded
	if err := meetsDeadline(*req); err != nil {
		return err
	}

	return nil
}

func Run(logger *log.Entry, req *pb.DeploymentRequest, cfg config.Config, kube kubeclient.TeamClientProvider, deployStatus chan *pb.DeploymentStatus) {
	var namespace string

	logger.Infof("Starting deployment")

	// Check the validity of the message.
	err := Prepare(req, cfg.Cluster)
	nl := logger.WithFields(req.LogFields())
	logger.Data = nl.Data // propagate changes down to caller

	if err != nil {
		if err != ErrNotMyCluster {
			logger.Tracef("Drop message: %s", err)
			deployStatus <- pb.NewFailureStatus(*req, err)
		} else {
			logger.Tracef("Drop message: running in %s, but deployment is addressed to %s", cfg.Cluster, req.GetCluster())
		}
		return
	}

	p := req.GetPayloadSpec()
	logger.Data["team"] = p.Team

	if cfg.TeamNamespaces {
		namespace = p.Team
	} else {
		namespace = DefaultTeamclientNamespace
	}

	teamClient, err := kube.TeamClient(p.Team, namespace, cfg.AutoCreateServiceAccount)
	if err != nil {
		deployStatus <- pb.NewErrorStatus(*req, err)
		return
	}

	rawResources, err := p.JSONResources()
	if err != nil {
		deployStatus <- pb.NewErrorStatus(*req, fmt.Errorf("unserializing kubernetes resources: %s", err))
		return
	}

	if len(rawResources) == 0 {
		deployStatus <- pb.NewErrorStatus(*req, fmt.Errorf("no resources to deploy"))
		return
	}

	resources, err := jsonToResources(rawResources)
	if err != nil {
		deployStatus <- pb.NewErrorStatus(*req, err)
		return
	}

	logger.Infof("Accepting incoming deployment request")

	wait := sync.WaitGroup{}
	errors := make(chan error, len(resources))

	for index, resource := range resources {
		addCorrelationID(&resource, req.GetDeliveryID())

		gvk := resource.GroupVersionKind().String()
		ns := resource.GetNamespace()
		n := resource.GetName()
		logger = logger.WithFields(log.Fields{
			"name":      n,
			"namespace": ns,
			"gvk":       gvk,
		})

		deployed, err := teamClient.DeployUnstructured(resource)
		if err != nil {
			err = fmt.Errorf("resource %d: %s", index+1, err)
			logger.Error(err)
			errors <- err
			break
		}

		metrics.KubernetesResources.Inc()

		logger.Infof("Resource %d: successfully deployed %s", index+1, deployed.GetSelfLink())

		go func(logger *log.Entry, resource unstructured.Unstructured) {
			wait.Add(1)
			logger.Infof("Monitoring rollout status of '%s/%s' in namespace '%s' for %s", gvk, n, ns, deploymentTimeout.String())
			err := teamClient.WaitForDeployment(logger, resource, time.Now().Add(deploymentTimeout))
			if err != nil {
				logger.Error(err)
				errors <- err
			}
			logger.Infof("Finished monitoring rollout status of '%s/%s' in namespace '%s'", gvk, n, ns)
			wait.Done()
		}(logger, resource)
	}

	deployStatus <- pb.NewInProgressStatus(*req)

	go func() {
		logger.Infof("Waiting for resources to be successfully rolled out")
		wait.Wait()
		logger.Infof("Finished monitoring all resources")

		errCount := len(errors)
		if errCount == 0 {
			deployStatus <- pb.NewSuccessStatus(*req)
		} else {
			err := <-errors
			deployStatus <- pb.NewFailureStatus(*req, fmt.Errorf("%s (total of %d errors)", err, errCount))
		}
	}()
}
