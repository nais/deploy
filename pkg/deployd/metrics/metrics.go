package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/nais/deploy/pkg/k8sutils"
)

const (
	namespace = "deployment"
	subsystem = "deployd"

	labelName      = "name"
	labelNamespace = "namespace"
	labelGvk       = "gvk"
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

	deployFieldValidationWarnings = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "deploy_field_validation_warnings",
		Help:      "number of deployments with field validation warnings",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			labelName,
			labelNamespace,
			labelGvk,
		},
	)
)

func DeployFieldValidationWarning(identifier k8sutils.Identifier) {
	labels := prometheus.Labels{
		labelName:      identifier.Name,
		labelNamespace: identifier.Namespace,
		labelGvk:       identifier.GroupVersionKind.String(),
	}
	deployFieldValidationWarnings.With(labels).Inc()
}

func init() {
	prometheus.MustRegister(DeploySuccessful)
	prometheus.MustRegister(DeployFailed)
	prometheus.MustRegister(DeployIgnored)
	prometheus.MustRegister(deployFieldValidationWarnings)
	prometheus.MustRegister(KubernetesResources)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
