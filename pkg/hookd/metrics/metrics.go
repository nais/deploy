package metrics

import (
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/nais/deploy/pkg/pb"
)

const (
	namespace = "deployment"
	subsystem = "hookd"

	StatusOK    = "ok"
	StatusError = "error"

	LabelStatus          = "status"
	LabelDeploymentState = "deployment_state"
	Repository           = "repository"
	Team                 = "team"
	Cluster              = "cluster"

	LabelType  = "type"
	LabelError = "error"
)

var (
	deployQueue        = make(map[string]interface{})
	clusterConnections = make(map[string]bool)
	qlock              = &sync.Mutex{}
)

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

	clusterStatus = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name:      "cluster_status",
		Help:      "0 if cluster is down, 1 if cluster is up",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			Cluster,
		},
	)

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

	interceptorRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "auth_interceptor_requests",
		Help:      "Number of requests by type in auth interceptor",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{LabelType, LabelError},
	)
)

func init() {
	prometheus.MustRegister(databaseQueries)
	prometheus.MustRegister(stateTransitions)
	prometheus.MustRegister(queueSize)
	prometheus.MustRegister(leadTime)
	prometheus.MustRegister(clusterStatus)
	prometheus.MustRegister(interceptorRequests)
}

func SetConnectedClusters(clusters []string) {
	for k := range clusterConnections {
		clusterConnections[k] = false
	}
	for _, k := range clusters {
		clusterConnections[k] = true
	}
	for k := range clusterConnections {
		i := 0.0
		if clusterConnections[k] {
			i = 1.0
		}
		clusterStatus.With(prometheus.Labels{
			Cluster: k,
		}).Set(i)
	}
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

func UpdateQueue(status *pb.DeploymentStatus) {
	labels := prometheus.Labels{
		LabelDeploymentState: status.GetState().String(),
		Repository:           status.GetRequest().GetRepository().FullName(),
		Team:                 status.GetRequest().GetTeam(),
		Cluster:              status.GetRequest().GetCluster(),
	}
	stateTransitions.With(labels).Inc()

	// avoid concurrent map writes
	qlock.Lock()
	defer qlock.Unlock()

	switch status.GetState() {

	// These three states are definite and signify the end of a deployment.
	case pb.DeploymentState_success:

		// In case of successful deployment, report the lead time.
		ttd := float64(time.Since(status.Timestamp()))
		leadTime.With(labels).Observe(ttd)

		fallthrough
	case pb.DeploymentState_inactive:
		fallthrough
	case pb.DeploymentState_error:
		fallthrough
	case pb.DeploymentState_failure:
		delete(deployQueue, status.GetRequest().GetID())

	// Other states mean the deployment is still being processed.
	default:
		deployQueue[status.GetRequest().GetID()] = new(interface{})
	}

	queueSize.Set(float64(len(deployQueue)))
}

func InterceptorRequest(requestType string, errType string) {
	interceptorRequests.With(prometheus.Labels{
		LabelType:  requestType,
		LabelError: errType,
	}).Inc()
}
