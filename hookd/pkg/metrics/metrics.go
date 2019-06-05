package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "deployment"
	subsystem = "hookd"
)

func counter(name, help string) prometheus.Counter {
	return prometheus.NewCounter(prometheus.CounterOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Subsystem: subsystem,
	})
}

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Subsystem: subsystem,
	})
}

var (
	WebhookRequests            = counter("webhook_requests", "number of incoming Github webhook requests")
	Dispatched                 = counter("dispatched", "number of deployment requests accepted and dispatched for deploy")
	GithubStatus               = counter("github_status", "number of Github status updates posted")
	GithubStatusFailed         = counter("github_status_failed", "number of Github status updates failed")
	KafkaQueueSize             = gauge("kafka_queue_size", "number of messages received from Kafka and waiting to be processed")
	DeploymentRequestQueueSize = gauge("deployment_request_queue_size", "number of github status updates waiting to be posted")
	GithubStatusQueueSize      = gauge("github_status_queue_size", "number of github status updates waiting to be posted")
)

func init() {
	prometheus.MustRegister(WebhookRequests)
	prometheus.MustRegister(Dispatched)
	prometheus.MustRegister(GithubStatus)
	prometheus.MustRegister(KafkaQueueSize)
	prometheus.MustRegister(DeploymentRequestQueueSize)
	prometheus.MustRegister(GithubStatusQueueSize)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
