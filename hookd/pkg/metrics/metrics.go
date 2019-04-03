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

var (
	WebhookRequests = counter("webhook_requests", "number of incoming Github webhook requests")
	Dispatched      = counter("dispatched", "number of deployment requests accepted and dispatched for deploy")
)

func init() {
	prometheus.MustRegister(WebhookRequests)
	prometheus.MustRegister(Dispatched)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
