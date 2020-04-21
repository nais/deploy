package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/navikt/deployment/common/pkg/deployment"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

const (
	namespace = "deployment"
	subsystem = "hookd"

	StatusOK    = "ok"
	StatusError = "error"

	LabelStatus          = "status"
	LabelStatusCode      = "status_code"
	LabelDeploymentState = "deployment_state"
	Repository           = "repository"
	Team                 = "team"
	Cluster              = "cluster"
)

var (
	deployQueue = make(map[string]interface{})
)

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      name,
		Help:      help,
		Namespace: namespace,
		Subsystem: subsystem,
	})
}

func GitHubRequest(statusCode int, repository, team string) {
	githubRequests.With(prometheus.Labels{
		LabelStatusCode: strconv.Itoa(statusCode),
		Repository:      repository,
		Team:            team,
	}).Inc()
}

func statusLabel(err error) string {
	if err == nil {
		return StatusOK
	}
	return StatusError
}

func DatabaseQuery(t time.Time, err error) {
	elapsed := time.Since(t)
	databaseQueries.With(prometheus.Labels{
		LabelStatus: statusLabel(err),
	}).Observe(elapsed.Seconds())
}

func UpdateQueue(status deployment.DeploymentStatus) {
	stateTransitions.With(prometheus.Labels{
		LabelDeploymentState: status.GetState().String(),
		Repository:           status.GetDeployment().GetRepository().FullName(),
		Team:                 status.GetTeam(),
		Cluster:              status.GetCluster(),
	}).Inc()

	switch status.GetState() {

	// These three states are definite and signify the end of a deployment.
	case deployment.GithubDeploymentState_success:

		// In case of successful deployment, report the lead time.
		ttd := float64(time.Now().Sub(status.Timestamp()))
		leadTime.With(prometheus.Labels{
			LabelDeploymentState: status.GetState().String(),
			Repository:           status.GetDeployment().GetRepository().FullName(),
			Team:                 status.GetTeam(),
			Cluster:              status.GetCluster(),
		}).Observe(ttd)

		fallthrough
	case deployment.GithubDeploymentState_error:
		fallthrough
	case deployment.GithubDeploymentState_failure:
		delete(deployQueue, status.GetDeliveryID())

	// Other states mean the deployment is still being processed.
	default:
		deployQueue[status.GetDeliveryID()] = new(interface{})
	}

	queueSize.Set(float64(len(deployQueue)))
}

var (
	databaseQueries = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:      "database_queries",
		Help:      "time to execute database queries",
		Namespace: namespace,
		Subsystem: subsystem,
		Buckets:   prometheus.LinearBuckets(0.005, 0.005, 20),
	},
		[]string{
			LabelStatus,
		},
	)

	githubRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "github_requests",
		Help:      "number of Github requests made",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
			Repository,
			Team,
		},
	)

	stateTransitions = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "state_transition",
		Help:      "deployment state transitions",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelDeploymentState,
			Repository,
			Team,
			Cluster,
		},
	)

	queueSize = prometheus.NewGauge(prometheus.GaugeOpts{
		Name:      "queue_size",
		Help:      "number of unfinished deployments",
		Namespace: namespace,
		Subsystem: subsystem,
	})

	leadTime = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Name:      "lead_time_seconds",
		Help:      "the time it takes from a deploy is made to it is running in the cluster",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelDeploymentState,
			Repository,
			Team,
			Cluster,
		},
	)

	KafkaQueueSize = gauge("kafka_queue_size", "number of messages received from Kafka and waiting to be processed")
)

func init() {
	prometheus.MustRegister(databaseQueries)
	prometheus.MustRegister(githubRequests)
	prometheus.MustRegister(stateTransitions)
	prometheus.MustRegister(queueSize)
	prometheus.MustRegister(leadTime)
	prometheus.MustRegister(KafkaQueueSize)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
