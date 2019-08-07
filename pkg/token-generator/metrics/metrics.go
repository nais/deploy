package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "token_generator"
	subsystem = ""

	LabelStatusCode = "status_code"
)

func TokenRequest(code int) {
	tokenRequests.With(prometheus.Labels{
		LabelStatusCode: strconv.Itoa(code),
	}).Inc()
}

func ApiKeyRequest(code int) {
	apiKeyRequests.With(prometheus.Labels{
		LabelStatusCode: strconv.Itoa(code),
	}).Inc()
}

var (
	tokenRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "token_requests",
		Help:      "number of incoming token issuer requests",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
		},
	)

	apiKeyRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "token_requests",
		Help:      "number of incoming token issuer requests",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
		},
	)
)

func init() {
	prometheus.MustRegister(tokenRequests)
	prometheus.MustRegister(apiKeyRequests)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
