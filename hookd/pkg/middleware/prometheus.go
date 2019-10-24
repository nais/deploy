// This code was originally written by Rene Zbinden and modified by Vladimir Konovalov.
// Copied from https://github.com/766b/chi-prometheus and further adapted.

package middleware

import (
	"net/http"
	"strconv"
	"time"

	chi_middleware "github.com/go-chi/chi/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	defaultBuckets = []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 15, 20, 30}
)

const (
	reqsName    = "requests_total"
	latencyName = "request_duration_ms"
)

type middleware func(http.Handler) http.Handler

// Middleware is a handler that exposes prometheus metrics for the number of requests,
// the latency and the response size, partitioned by status code, method and HTTP path.
type Middleware struct {
	reqs    *prometheus.CounterVec
	latency *prometheus.HistogramVec
}

// NewMiddleware returns a new prometheus Middleware handler.
func PrometheusMiddlewareHandler(name string, buckets ...float64) middleware {
	var m Middleware
	m.reqs = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        reqsName,
			Help:        "How many HTTP requests processed, partitioned by status code, method and HTTP path.",
			ConstLabels: prometheus.Labels{"service": name},
		},
		[]string{"code", "method", "path"},
	)

	if len(buckets) == 0 {
		buckets = defaultBuckets
	}
	m.latency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:        latencyName,
		Help:        "How long it took to process the request, partitioned by status code, method and HTTP path.",
		ConstLabels: prometheus.Labels{"service": name},
		Buckets:     buckets,
	},
		[]string{"code", "method", "path"},
	)

	prometheus.MustRegister(m.reqs)
	prometheus.MustRegister(m.latency)

	return m.handler
}

func (c Middleware) handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := chi_middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		statusCode := strconv.Itoa(ww.Status())
		duration := time.Since(start)
		c.reqs.WithLabelValues(statusCode, r.Method, r.URL.Path).Inc()
		c.latency.WithLabelValues(statusCode, r.Method, r.URL.Path).Observe(duration.Seconds())
	}
	return http.HandlerFunc(fn)
}
