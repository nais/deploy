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

func WebhookRequest(code int) {
	webhookRequests.With(prometheus.Labels{
		LabelStatusCode: strconv.Itoa(code),
	}).Inc()
}

func DeploymentStatus(status deployment.DeploymentStatus, githubReturnCode int) {
	githubStatus.With(prometheus.Labels{
		LabelStatusCode:      strconv.Itoa(githubReturnCode),
		LabelDeploymentState: status.GetState().String(),
		Repository:           status.GetDeployment().GetRepository().FullName(),
		Team:                 status.GetTeam(),
		Cluster:              status.GetCluster(),
	}).Inc()

	if status.GetState() != deployment.GithubDeploymentState_success || githubReturnCode > 299 {
		return
	}

	ttd := float64(time.Now().Unix() - status.GetTimestamp())

	leadTime.With(prometheus.Labels{
		LabelStatusCode:      strconv.Itoa(githubReturnCode),
		LabelDeploymentState: status.GetState().String(),
		Repository:           status.GetDeployment().GetRepository().FullName(),
		Team:                 status.GetTeam(),
		Cluster:              status.GetCluster(),
	}).Observe(ttd)
}

func UpdateQueue(status deployment.DeploymentStatus) {
	switch status.GetState() {
	// These three states are definite and signify the end of a deployment.
	case deployment.GithubDeploymentState_success:
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
	webhookRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "webhook_requests",
		Help:      "number of incoming Github webhook requests",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
		},
	)

	githubStatus = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "github_status",
		Help:      "number of Github status updates posted",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
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
			LabelStatusCode,
			LabelDeploymentState,
			Repository,
			Team,
			Cluster,
		},
	)

	Dispatched = prometheus.NewCounter(prometheus.CounterOpts{
		Name:      "dispatched",
		Help:      "number of deployment requests dispatched to Kafka",
		Namespace: namespace,
		Subsystem: subsystem,
	})

	VaultTokenRefresh = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name:      "vault_token_refresh",
		Help:      "number of token refresh calls made to Vault",
		Namespace: namespace,
		Subsystem: subsystem,
	},
		[]string{
			LabelStatusCode,
		},
	)

	KafkaQueueSize             = gauge("kafka_queue_size", "number of messages received from Kafka and waiting to be processed")
	DeploymentRequestQueueSize = gauge("deployment_request_queue_size", "number of github status updates waiting to be posted")
	GithubStatusQueueSize      = gauge("github_status_queue_size", "number of github status updates waiting to be posted")
)

func init() {
	prometheus.MustRegister(webhookRequests)
	prometheus.MustRegister(githubStatus)
	prometheus.MustRegister(queueSize)
	prometheus.MustRegister(leadTime)
	prometheus.MustRegister(Dispatched)
	prometheus.MustRegister(VaultTokenRefresh)
	prometheus.MustRegister(KafkaQueueSize)
	prometheus.MustRegister(DeploymentRequestQueueSize)
	prometheus.MustRegister(GithubStatusQueueSize)
}

func Handler() http.Handler {
	return promhttp.Handler()
}
