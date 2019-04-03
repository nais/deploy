package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "deployment"
	subsystem = "deployd"
)

func counter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Subsystem: subsystem,
	})
}

var (
	DeploySuccessful    = counter("deploy_successful", "number of successful deployments")
	DeployFailed        = counter("deploy_failed", "number of failed deployments")
	DeployIgnored       = counter("deploy_ignored", "number of ignored/discarded deployments")
	KubernetesResources = counter("kubernetes_resources", "number of Kubernetes resources successfully committed to cluster")
)

func init() {
	prometheus.MustRegister(DeploySuccessful)
	prometheus.MustRegister(DeployFailed)
	prometheus.MustRegister(DeployIgnored)
	prometheus.MustRegister(KubernetesResources)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
