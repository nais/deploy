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
	kubernetesResources = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "kubernetes_resources",
		Help:      "number of Kubernetes resources successfully committed to cluster",
		Namespace: namespace,
		Subsystem: subsystem,
	}, []string{
		"team",
		"kind",
		"name",
	})
)

func KubernetesResources(team, kind, name string) prometheus.Counter {
	return kubernetesResources.WithLabelValues(team, kind, name)
}

func init() {
	prometheus.MustRegister(DeploySuccessful)
	prometheus.MustRegister(DeployFailed)
	prometheus.MustRegister(DeployIgnored)
	prometheus.MustRegister(kubernetesResources)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
